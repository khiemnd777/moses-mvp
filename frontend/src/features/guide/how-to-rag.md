# Hướng dẫn RAG

Tài liệu này dùng:

- Nội dung thật từ file mẫu `52.2014.QH13.docx`
- Cấu trúc thật của hệ thống hiện tại

Nếu dữ liệu không có trong file hoặc không có trong cấu trúc hệ thống đã đọc được, tài liệu sẽ ghi rõ `Thiếu thông tin`.

## 1. Giải thích DocType

DocType là mẫu chung để hệ thống hiểu một tài liệu thuộc loại gì.

Trong hệ thống này, DocType không chỉ có tên loại văn bản, mà còn có một `form` để mô tả cách hệ thống nhận diện, lưu các trường thông tin và xử lý tài liệu đó.

Vai trò của DocType trong hệ thống:

- Giúp hệ thống biết đây là loại tài liệu nào
- Xác định các thông tin cần lưu cho loại tài liệu đó
- Quy định cách chia cấu trúc văn bản như chương, điều, khoản, điểm
- Giúp Document phía sau tham chiếu đúng loại tài liệu

### Ví dụ DocType (JSON)

Ví dụ dưới đây bám theo đúng cấu trúc `DocTypeForm` của hệ thống và dùng dữ liệu thật từ file `52.2014.QH13.docx`.

```json
{
  "version": 1,
  "doc_type": {
    "code": "vn_marriage_family_law",
    "name": "Vietnam Marriage & Family Law"
  },
  "segment_rules": {
    "strategy": "legal_article",
    "hierarchy": "chapter.article.clause.point",
    "normalization": "trim_whitespace_preserve_numbering",
    "level_patterns": {
      "article": "(?im)^\\s*Điều\\s+([0-9]+)",
      "chapter": "(?im)^\\s*CHƯƠNG\\s+([IVXLCDM]+)",
      "clause": "(?m)^\\s*([0-9]+)\\.\\s",
      "point": "(?m)^\\s*([a-zđ])\\)\\s"
    }
  },
  "metadata_schema": {
    "fields": [
      {
        "name": "document_number",
        "type": "string"
      },
      {
        "name": "document_type",
        "type": "string"
      },
      {
        "name": "issuing_authority",
        "type": "string"
      },
      {
        "name": "signed_date",
        "type": "date"
      },
      {
        "name": "effective_date",
        "type": "date"
      },
      {
        "name": "legal_domain",
        "type": "string"
      }
    ]
  },
  "mapping_rules": [
    {
      "field": "document_number",
      "regex": "(?im)\\b(?:Luật\\s+số|Số)\\s*:?\\s*([0-9]+/[0-9]+/QH[0-9]+)",
      "group": 1,
      "default": "unknown"
    },
    {
      "field": "document_type",
      "regex": "(?im)^\\s*(LUẬT)\\s*$",
      "group": 1,
      "default": "Luật"
    },
    {
      "field": "issuing_authority",
      "regex": "(?im)^\\s*(QUỐC HỘI)\\s*$",
      "group": 1,
      "default": "Quốc hội"
    },
    {
      "field": "signed_date",
      "regex": "(?im)thông\\s+qua\\s+ngày\\s+(\\d{1,2})\\s+tháng\\s+(\\d{1,2})\\s+năm\\s+(\\d{4})",
      "group": 0,
      "default": ""
    },
    {
      "field": "effective_date",
      "regex": "(?im)có\\s+hiệu\\s+lực\\s+thi\\s+hành\\s+từ\\s+ngày\\s+(\\d{1,2})\\s+tháng\\s+(\\d{1,2})\\s+năm\\s+(\\d{4})",
      "group": 0,
      "default": ""
    },
    {
      "field": "legal_domain",
      "regex": "(?im)hôn\\s+nhân|gia\\s+đình|kết\\s+hôn|ly\\s+hôn|nuôi\\s+con",
      "group": 0,
      "default": "marriage_family"
    }
  ],
  "reindex_policy": {
    "on_content_change": true,
    "on_form_change": true
  }
}
```

## 2. Giải thích Document

Document là bản ghi của một tài liệu cụ thể trong hệ thống.

Quan hệ giữa Document và DocType:

- DocType mô tả loại tài liệu
- Document là tài liệu thật thuộc loại đó
- Một Document sẽ gắn với `doc_type_code`

Trong code hiện tại, khi tạo Document mới, hệ thống yêu cầu ít nhất:

- `doc_type_code`
- `title`

Sau khi được tạo trong hệ thống, Document còn có thêm các thông tin như `id`, `doc_type_id`, `assets`, `created_at`, `updated_at`.

### Ví dụ Document (JSON)

Ví dụ dưới đây dùng đúng tên file thật từ input.

```json
{
  "id": "Thiếu thông tin",
  "doc_type_id": "Thiếu thông tin",
  "doc_type_code": "vn_marriage_family_law",
  "title": "Luật Hôn nhân và gia đình",
  "assets": [
    {
      "file_name": "52.2014.QH13.docx",
      "content_type": "Thiếu thông tin",
      "created_at": "21/03/2026",
      "versions": [1]
    }
  ],
  "created_at": "21/03/2026",
  "updated_at": "21/03/2026"
}
```

## 3. Guard Policies

Guard là các quy tắc an toàn cho câu trả lời của hệ thống AI.

Vai trò của Guard:

- Giúp hệ thống không trả lời vượt quá dữ liệu đã có
- Yêu cầu hệ thống thận trọng khi nguồn ít hoặc độ tin cậy thấp
- Giảm rủi ro trả lời sai từ tài liệu pháp luật

Trong hệ thống này, danh sách Guard Policies trả về theo dạng có `items`, và mỗi policy có đầy đủ các trường sau:

- `name`
- `enabled`
- `min_retrieved_chunks`
- `min_similarity_score`
- `on_empty_retrieval`
- `on_low_confidence`

### Ví dụ Guard Policies (JSON)

```json
{
  "items": [
    {
      "id": "Thiếu thông tin",
      "name": "Guard cho văn bản luật",
      "enabled": true,
      "min_retrieved_chunks": 1,
      "min_similarity_score": 0.7,
      "on_empty_retrieval": "refuse",
      "on_low_confidence": "ask_clarification",
      "created_at": "21/03/2026",
      "updated_at": "21/03/2026"
    }
  ]
}
```

## 4. System Prompts

System Prompt là phần chỉ dẫn nền để AI biết phải trả lời theo cách nào.

Trong hệ thống này, System Prompt giúp AI:

- Chỉ bám vào tài liệu đã nạp
- Không tự thêm nội dung ngoài văn bản
- Trả lời rõ ràng khi thiếu dữ liệu

### Ví dụ System Prompt

Ví dụ dưới đây bám đúng văn bản đã cung cấp và dùng đúng cấu trúc dữ liệu danh sách `AIPrompt` mà hệ thống trả về.

```json
{
  "items": [
    {
      "id": "Thiếu thông tin",
      "name": "Prompt cho Luật Hôn nhân và gia đình",
      "prompt_type": "legal_guard",
      "system_prompt": "Bạn là trợ lý Legal RAG. Chỉ sử dụng thông tin có trong file 52.2014.QH13.docx. Đây là Luật Hôn nhân và gia đình, Luật số 52/2014/QH13, do Quốc hội thông qua ngày 19 tháng 6 năm 2014 và có hiệu lực từ ngày 01 tháng 01 năm 2015. Không được tự thêm dữ liệu ngoài nội dung văn bản. Nếu nguồn không có thông tin thì trả lời: \"Thiếu thông tin\".",
      "temperature": 0.2,
      "max_tokens": 1200,
      "retry": 2,
      "enabled": true,
      "created_at": "21/03/2026",
      "updated_at": "21/03/2026"
    }
  ]
}
```

## 5. Ví dụ thực tế

### file_name thật

`52.2014.QH13.docx`

### Trích 1 đoạn nội dung thật

> Luật này quy định chế độ hôn nhân và gia đình; chuẩn mực pháp lý cho cách ứng xử giữa các thành viên gia đình; trách nhiệm của cá nhân, tổ chức, Nhà nước và xã hội trong việc xây dựng, củng cố chế độ hôn nhân và gia đình.

### DocType tương ứng

```json
{
  "version": 1,
  "doc_type": {
    "code": "vn_marriage_family_law",
    "name": "Vietnam Marriage & Family Law"
  },
  "segment_rules": {
    "strategy": "legal_article",
    "hierarchy": "chapter.article.clause.point",
    "normalization": "trim_whitespace_preserve_numbering",
    "level_patterns": {
      "article": "(?im)^\\s*Điều\\s+([0-9]+)",
      "chapter": "(?im)^\\s*CHƯƠNG\\s+([IVXLCDM]+)",
      "clause": "(?m)^\\s*([0-9]+)\\.\\s",
      "point": "(?m)^\\s*([a-zđ])\\)\\s"
    }
  },
  "metadata_schema": {
    "fields": [
      {
        "name": "document_number",
        "type": "string"
      },
      {
        "name": "document_type",
        "type": "string"
      }
    ]
  },
  "mapping_rules": [
    {
      "field": "document_number",
      "regex": "(?im)\\b(?:Luật\\s+số|Số)\\s*:?\\s*([0-9]+/[0-9]+/QH[0-9]+)",
      "group": 1,
      "default": "unknown"
    },
    {
      "field": "document_type",
      "regex": "(?im)^\\s*(LUẬT)\\s*$",
      "group": 1,
      "default": "Luật"
    }
  ],
  "reindex_policy": {
    "on_content_change": true,
    "on_form_change": true
  }
}
```

### Document tương ứng

```json
{
  "id": "Thiếu thông tin",
  "doc_type_id": "Thiếu thông tin",
  "doc_type_code": "vn_marriage_family_law",
  "title": "Luật Hôn nhân và gia đình",
  "assets": [
    {
      "file_name": "52.2014.QH13.docx",
      "content_type": "Thiếu thông tin",
      "created_at": "21/03/2026",
      "versions": [1]
    }
  ],
  "created_at": "21/03/2026",
  "updated_at": "21/03/2026"
}
```
