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
	return nil
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
