package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type DocTypeForm struct {
	Version       int            `json:"version"`
	DocType       DocType        `json:"doc_type"`
	SegmentRules  SegmentRules   `json:"segment_rules"`
	Metadata      MetadataSchema `json:"metadata_schema"`
	MappingRules  []MappingRule  `json:"mapping_rules"`
	ReindexPolicy ReindexPolicy  `json:"reindex_policy"`
	QueryProfile  QueryProfile   `json:"query_profile,omitempty"`
}

type DocType struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type SegmentRules struct {
	Strategy      string            `json:"strategy"`
	Hierarchy     string            `json:"hierarchy"`
	Normalization string            `json:"normalization"`
	LevelPatterns map[string]string `json:"level_patterns,omitempty"`
}

func (r SegmentRules) HierarchyLevels() []string {
	parts := splitHierarchyLevels(r.Hierarchy)
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		out = append(out, part)
	}
	return out
}

type MetadataSchema struct {
	Fields []MetadataField `json:"fields"`
}

type MetadataField struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type MappingRule struct {
	Field    string            `json:"field"`
	Regex    string            `json:"regex"`
	Group    int               `json:"group"`
	Default  string            `json:"default"`
	ValueMap map[string]string `json:"value_map,omitempty"`
}

type ReindexPolicy struct {
	OnContentChange bool `json:"on_content_change"`
	OnFormChange    bool `json:"on_form_change"`
}

type QueryProfile struct {
	CanonicalTerms    []string          `json:"canonical_terms,omitempty"`
	SynonymGroups     []SynonymGroup    `json:"synonym_groups,omitempty"`
	QuerySignals      []string          `json:"query_signals,omitempty"`
	IntentRules       []IntentRule      `json:"intent_rules,omitempty"`
	DomainTopicRules  []DomainTopicRule `json:"domain_topic_rules,omitempty"`
	LegalSignalRules  []string          `json:"legal_signal_rules,omitempty"`
	FollowUpMarkers   []string          `json:"followup_markers,omitempty"`
	PreferredDocTypes []string          `json:"preferred_doc_types,omitempty"`
	RoutingPriority   int               `json:"routing_priority,omitempty"`
}

type SynonymGroup struct {
	Canonical string   `json:"canonical"`
	Aliases   []string `json:"aliases,omitempty"`
}

type IntentRule struct {
	Intent string   `json:"intent"`
	Terms  []string `json:"terms,omitempty"`
}

type DomainTopicRule struct {
	LegalDomain string   `json:"legal_domain"`
	LegalTopic  string   `json:"legal_topic,omitempty"`
	Terms       []string `json:"terms,omitempty"`
}

func (f DocTypeForm) AlignMappingRules() DocTypeForm {
	c := f
	existing := make(map[string]MappingRule, len(c.MappingRules))
	for _, rule := range c.MappingRules {
		field := strings.TrimSpace(rule.Field)
		if field == "" {
			continue
		}
		if _, ok := existing[field]; ok {
			continue
		}
		rule.Field = field
		if rule.Group < 0 {
			rule.Group = 0
		}
		existing[field] = rule
	}

	aligned := make([]MappingRule, 0, len(c.Metadata.Fields))
	for _, field := range c.Metadata.Fields {
		name := strings.TrimSpace(field.Name)
		if name == "" {
			continue
		}
		if rule, ok := existing[name]; ok {
			rule.Field = name
			aligned = append(aligned, rule)
			continue
		}
		aligned = append(aligned, MappingRule{
			Field:   name,
			Regex:   "",
			Group:   1,
			Default: "",
		})
	}
	c.MappingRules = aligned
	return c
}

func (f DocTypeForm) Validate() error {
	if f.Version <= 0 {
		return errors.New("version must be > 0")
	}
	if f.DocType.Code == "" || f.DocType.Name == "" {
		return errors.New("doc_type.code and doc_type.name are required")
	}
	if f.SegmentRules.Strategy == "" {
		return errors.New("segment_rules.strategy is required")
	}
	rawHierarchy := splitHierarchyLevels(f.SegmentRules.Hierarchy)
	if strings.TrimSpace(f.SegmentRules.Hierarchy) != "" && strings.TrimSpace(strings.ToLower(f.SegmentRules.Hierarchy)) != "none" && len(rawHierarchy) == 0 {
		return errors.New("segment_rules.hierarchy must contain at least one non-empty level")
	}
	seenLevels := make(map[string]struct{}, len(rawHierarchy))
	for _, level := range rawHierarchy {
		if _, ok := seenLevels[level]; ok {
			return fmt.Errorf("segment_rules.hierarchy level %q is duplicated", level)
		}
		seenLevels[level] = struct{}{}
	}
	for level, pattern := range f.SegmentRules.LevelPatterns {
		if strings.TrimSpace(level) == "" || strings.TrimSpace(pattern) == "" {
			return errors.New("segment_rules.level_patterns keys and values must be non-empty")
		}
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("segment_rules.level_patterns[%q] is invalid: %v", level, err)
		}
	}
	if len(f.Metadata.Fields) == 0 {
		return errors.New("metadata_schema.fields is required")
	}
	fieldNames := make(map[string]struct{}, len(f.Metadata.Fields))
	for _, field := range f.Metadata.Fields {
		if field.Name == "" || field.Type == "" {
			return errors.New("metadata field name/type are required")
		}
		if _, exists := fieldNames[field.Name]; exists {
			return fmt.Errorf("metadata field %q is duplicated", field.Name)
		}
		fieldNames[field.Name] = struct{}{}
	}
	if len(f.MappingRules) == 0 {
		return errors.New("mapping_rules is required and must align with metadata_schema.fields")
	}
	ruleByField := make(map[string]int, len(f.MappingRules))
	for i, rule := range f.MappingRules {
		if strings.TrimSpace(rule.Field) == "" {
			return fmt.Errorf("mapping_rules[%d].field is required", i)
		}
		if rule.Group < 0 {
			return fmt.Errorf("mapping_rules[%d].group must be >= 0", i)
		}
		if _, ok := fieldNames[rule.Field]; !ok {
			return fmt.Errorf("mapping_rules[%d].field %q is not defined in metadata_schema.fields", i, rule.Field)
		}
		if strings.TrimSpace(rule.Regex) != "" {
			if _, err := regexp.Compile(rule.Regex); err != nil {
				return fmt.Errorf("mapping_rules[%d].regex is invalid: %v", i, err)
			}
		}
		ruleByField[rule.Field]++
	}
	for _, field := range f.Metadata.Fields {
		if ruleByField[field.Name] == 0 {
			return fmt.Errorf("metadata field %q has no mapping rule", field.Name)
		}
	}
	if err := f.QueryProfile.Validate(); err != nil {
		return err
	}
	return nil
}

func (p QueryProfile) Validate() error {
	seenTerms := map[string]string{}
	for _, term := range p.CanonicalTerms {
		if err := validateProfileTerm("canonical_terms", term); err != nil {
			return err
		}
	}
	for idx, group := range p.SynonymGroups {
		if err := validateProfileTerm(fmt.Sprintf("synonym_groups[%d].canonical", idx), group.Canonical); err != nil {
			return err
		}
		registerProfileTerm(seenTerms, group.Canonical, fmt.Sprintf("synonym_groups[%d].canonical", idx))
		for aliasIdx, alias := range group.Aliases {
			if err := validateProfileTerm(fmt.Sprintf("synonym_groups[%d].aliases[%d]", idx, aliasIdx), alias); err != nil {
				return err
			}
			if prior, ok := seenTerms[normalizeProfileTerm(alias)]; ok {
				return fmt.Errorf("query_profile duplicate alias %q conflicts with %s", alias, prior)
			}
			registerProfileTerm(seenTerms, alias, fmt.Sprintf("synonym_groups[%d].aliases[%d]", idx, aliasIdx))
		}
	}
	for idx, signal := range p.QuerySignals {
		if err := validateProfileTerm(fmt.Sprintf("query_signals[%d]", idx), signal); err != nil {
			return err
		}
	}
	for idx, rule := range p.IntentRules {
		if strings.TrimSpace(rule.Intent) == "" {
			return fmt.Errorf("query_profile intent_rules[%d].intent is required", idx)
		}
		if len(rule.Terms) == 0 {
			return fmt.Errorf("query_profile intent_rules[%d].terms is required", idx)
		}
		for termIdx, term := range rule.Terms {
			if err := validateProfileTerm(fmt.Sprintf("intent_rules[%d].terms[%d]", idx, termIdx), term); err != nil {
				return err
			}
		}
	}
	for idx, rule := range p.DomainTopicRules {
		if strings.TrimSpace(rule.LegalDomain) == "" {
			return fmt.Errorf("query_profile domain_topic_rules[%d].legal_domain is required", idx)
		}
		if len(rule.Terms) == 0 {
			return fmt.Errorf("query_profile domain_topic_rules[%d].terms is required", idx)
		}
		for termIdx, term := range rule.Terms {
			if err := validateProfileTerm(fmt.Sprintf("domain_topic_rules[%d].terms[%d]", idx, termIdx), term); err != nil {
				return err
			}
		}
	}
	for idx, signal := range p.LegalSignalRules {
		if err := validateProfileTerm(fmt.Sprintf("legal_signal_rules[%d]", idx), signal); err != nil {
			return err
		}
	}
	for idx, marker := range p.FollowUpMarkers {
		if err := validateProfileTerm(fmt.Sprintf("followup_markers[%d]", idx), marker); err != nil {
			return err
		}
	}
	for idx, docType := range p.PreferredDocTypes {
		if err := validateProfileTerm(fmt.Sprintf("preferred_doc_types[%d]", idx), docType); err != nil {
			return err
		}
	}
	if p.RoutingPriority < 0 {
		return errors.New("query_profile.routing_priority must be >= 0")
	}
	return nil
}

func validateProfileTerm(path, value string) error {
	if normalizeProfileTerm(value) == "" {
		return fmt.Errorf("query_profile %s must be non-empty", path)
	}
	return nil
}

func registerProfileTerm(seen map[string]string, value, path string) {
	key := normalizeProfileTerm(value)
	if key == "" {
		return
	}
	if _, ok := seen[key]; !ok {
		seen[key] = path
	}
}

func normalizeProfileTerm(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(strings.ToLower(value))), " ")
}

func splitHierarchyLevels(hierarchy string) []string {
	hierarchy = strings.TrimSpace(strings.ToLower(hierarchy))
	if hierarchy == "" || hierarchy == "none" {
		return nil
	}
	parts := strings.FieldsFunc(hierarchy, func(ch rune) bool {
		return ch == '>' || ch == ',' || ch == '/' || ch == '|' || ch == ';' || ch == '.'
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func (f DocTypeForm) Hash() (string, error) {
	c := f.canonical()
	b, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}

func (f DocTypeForm) canonical() DocTypeForm {
	c := f
	sort.Slice(c.Metadata.Fields, func(i, j int) bool {
		return c.Metadata.Fields[i].Name < c.Metadata.Fields[j].Name
	})
	sort.Slice(c.MappingRules, func(i, j int) bool {
		if c.MappingRules[i].Field != c.MappingRules[j].Field {
			return c.MappingRules[i].Field < c.MappingRules[j].Field
		}
		if c.MappingRules[i].Regex != c.MappingRules[j].Regex {
			return c.MappingRules[i].Regex < c.MappingRules[j].Regex
		}
		if c.MappingRules[i].Group != c.MappingRules[j].Group {
			return c.MappingRules[i].Group < c.MappingRules[j].Group
		}
		return c.MappingRules[i].Default < c.MappingRules[j].Default
	})
	return c
}
