# Hướng dẫn cài đặt ChatGPT

Mục này tập trung vào hướng dẫn cấu hình ChatGPT Project bằng 2 file markdown sau đây:

- [`chatgpt_legal_rag_instruction.md`](guide-source://chatgpt_legal_rag_instruction)
- [`doctype_generator_spec.md`](guide-source://doctype_generator_spec)

`Copy` nội dung hai file này và paste vào `Sources` của ChatGPT's Project để nó hiểu cách phân tích tài liệu pháp lý theo đúng cấu trúc hệ thống.

## Bước 1: Tạo mới ChatGPT Project

Đăng nhập ChatGPT và tạo một project mới để gom toàn bộ source, file test và lịch sử trao đổi vào cùng một nơi.

## Bước 2: Đặt tên Project `RAG AI LEGAL`

Tên project nên đặt chính xác là:

```plaintext
RAG AI LEGAL
```

Việc dùng cùng một tên giúp team dễ nhận biết đây là project phục vụ phân tích tài liệu cho luồng Legal RAG.

## Bước 3: Mở tab `Sources`

Sau khi tạo project, mở tab `Sources` để quản lý toàn bộ tài liệu nền mà ChatGPT sẽ dùng khi phân tích file pháp lý.

## Bước 4: Upload hoặc tạo mới source [`chatgpt_legal_rag_instruction.md`](guide-source://chatgpt_legal_rag_instruction)

Thêm file:

```plaintext
chatgpt_legal_rag_instruction.md
```

Mục đích của file này là cung cấp instruction tổng quát để ChatGPT:

- hiểu vai trò của mình trong luồng Legal RAG
- không tự ý suy diễn ngoài dữ liệu được cung cấp
- trả về đầu ra có cấu trúc phục vụ DocType và Document

## Bước 5: Upload hoặc tạo mới source [`doctype_generator_spec.md`](guide-source://doctype_generator_spec)

Tiếp tục thêm file:

```plaintext
doctype_generator_spec.md
```

Mục đích của file này là mô tả specification chi tiết cho việc sinh:

- `DocType`
- `DocType form JSON`
- `Document`
- các placeholder liên quan đến `Asset` và `Version`

## Bước 6: Test thử với file `52.2014.QH13.docx`

Thực hiện test nhanh để xác nhận project đã dùng đúng sources.

### 1. Upload file `52.2014.QH13.docx`

Upload file tài liệu pháp lý mẫu:

```plaintext
52.2014.QH13.docx
```

### 2. Nhập prompt sau

```plaintext
Phân tích file thành DocType và Document
```

## Bước 7: Xem kết quả sẽ ra tương tự

- Doc Type:

```json
{
  "version": 1,
  "doc_type": {
    "code": "vn_circular_marriage_family_foreign_2015",
    "name": "Vietnam Circular - Marriage & Family with Foreign Elements"
  },
  "segment_rules": {
    "strategy": "legal_article",
    "hierarchy": "chapter.article.clause.point",
    "normalization": "trim_whitespace_preserve_numbering",
    "level_patterns": {
      "chapter": "(?im)^\\s*(?:Chương|CHƯƠNG)\\s+([IVXLCDM]+)",
      "article": "(?im)^\\s*Điều\\s+([0-9]+)",
      "clause": "(?m)^\\s*([0-9]+)\\.\\s",
      "point": "(?m)^\\s*([a-zđ])\\)\\s"
    }
  },
  "metadata_schema": {
    "fields": [
      { "name": "document_number", "type": "string" },
      { "name": "document_type", "type": "string" },
      { "name": "issuing_authority", "type": "string" },
      { "name": "signed_date", "type": "string" },
      { "name": "effective_date", "type": "string" },
      { "name": "legal_domain", "type": "string" }
    ]
  },
  "mapping_rules": [
    {
      "field": "document_number",
      "regex": "(?i)Số\\s*:\\s*([0-9a-zA-Z\\-\\/]+)",
      "group": 1,
      "default": "unknown"
    },
    {
      "field": "document_type",
      "regex": "(?i)THÔNG TƯ",
      "group": 0,
      "default": "circular",
      "value_map": {
        "THÔNG TƯ": "circular"
      }
    },
    {
      "field": "issuing_authority",
      "regex": "(?im)^\\s*(BỘ\\s+TƯ\\s+PHÁP)",
      "group": 1,
      "default": "unknown"
    },
    {
      "field": "signed_date",
      "regex": "(?i)ngày\\s+([0-9]{1,2}\\s+tháng\\s+[0-9]{1,2}\\s+năm\\s+[0-9]{4})",
      "group": 1,
      "default": ""
    },
    {
      "field": "effective_date",
      "regex": "(?i)có hiệu lực từ ngày\\s+([0-9]{1,2}\\s+tháng\\s+[0-9]{1,2}\\s+năm\\s+[0-9]{4})",
      "group": 1,
      "default": ""
    },
    {
      "field": "legal_domain",
      "regex": "(?i)hôn nhân và gia đình",
      "group": 0,
      "default": "marriage_family",
      "value_map": {
        "hôn nhân và gia đình": "marriage_family"
      }
    }
  ],
  "reindex_policy": {
    "on_content_change": true,
    "on_form_change": true
  },
  "query_profile": {
    "canonical_terms": [
      "kết hôn",
      "nhận cha mẹ con",
      "tình trạng hôn nhân",
      "hôn nhân có yếu tố nước ngoài"
    ],
    "synonym_groups": [
      ["đăng ký kết hôn", "kết hôn"],
      ["xác nhận tình trạng hôn nhân", "giấy độc thân"],
      ["nhận cha mẹ con", "xác nhận quan hệ huyết thống"]
    ],
    "query_signals": [
      "thủ tục kết hôn",
      "đăng ký kết hôn với người nước ngoài",
      "giấy xác nhận tình trạng hôn nhân"
    ],
    "intent_rules": [
      "how_to_register_marriage",
      "marriage_foreign_procedure",
      "civil_status_certificate"
    ],
    "domain_topic_rules": [
      "marriage_foreign",
      "civil_status"
    ],
    "legal_signal_rules": [
      "Điều",
      "Chương",
      "Thông tư"
    ],
    "followup_markers": [
      "cần giấy gì",
      "thủ tục ra sao",
      "bao lâu"
    ],
    "preferred_doc_types": [
      "vn_circular_marriage_family_foreign_2015"
    ],
    "routing_priority": 90
  }
}
```

- Document

```json
{
  "title": "Thông tư số 02a/2015/TT-BTP hướng dẫn thi hành Nghị định 126/2014/NĐ-CP về hôn nhân và gia đình có yếu tố nước ngoài",
  "doc_type_code": "vn_circular_marriage_family_foreign_2015"
}
```

- Actions

```json
{
  "asset": {
    "file_name": "4-HNGD-02a.2015.TT.BTP.docx",
    "content_type": "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
  },
  "version": {
    "version": 1
  }
}
```

## Ghi chú

- Nếu ChatGPT chưa trả về đúng cấu trúc, kiểm tra lại tab `Sources` xem đã nạp đủ 2 file markdown chưa.
- Nếu kết quả chỉ tóm tắt nội dung văn bản mà không sinh `DocType` và `Document`, thường là do instruction source chưa được dùng đúng.
- Sau bước này, có thể chuyển sang mục `Hướng dẫn RAG` để đối chiếu lại vai trò của `DocType`, `Document`, `Guard Policy` và `System Prompt` trong hệ thống.
