# Luồng RAG hoạt động thế nào?

Tài liệu này mô tả luồng RAG sẽ hoạt động như thế nào.

Nguồn tham chiếu:

- File mẫu thật `52.2014.QH13.docx`
- Cấu trúc dữ liệu và màn hình hiện có của hệ thống

Nếu một giá trị không xác định được từ tài liệu mẫu hoặc từ cấu trúc hiện tại, nội dung sẽ ghi là `Thiếu thông tin`.

## 1. Tổng quan luồng RAG

Trong hệ thống này, luồng RAG đi theo thứ tự:

1. Xác định `DocType` cho tài liệu.
2. Tạo `Document` đại diện cho file thực tế.
3. Thiết lập `Guard Policy` để giới hạn hành vi trả lời.
4. Thiết lập `System Prompt` để chuẩn hóa cách model phản hồi.
5. Đưa dữ liệu vào ingest để phục vụ truy xuất và trả lời.

Phần dưới đây mô tả từng thành phần theo đúng cách hệ thống đang tổ chức dữ liệu.

## 2. DocType

`DocType` là mẫu chuẩn để hệ thống hiểu tài liệu thuộc nhóm nào và phải được bóc tách ra sao.

Vai trò chính:

- Gắn mã loại tài liệu để dùng lại cho nhiều văn bản cùng nhóm
- Mô tả schema metadata cần lưu
- Xác định luật chia đoạn như chương, điều, khoản, điểm
- Làm nền cho ingest và truy xuất sau này

### Ví dụ DocType

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

## 3. Document

`Document` là bản ghi đại diện cho một tài liệu cụ thể.

Quan hệ với `DocType`:

- `DocType` mô tả mẫu chung
- `Document` là file cụ thể đang được xử lý
- Mỗi `Document` tham chiếu đến một `doc_type_code`

Khi tạo mới, các trường tối thiểu là:

- `doc_type_code`
- `title`

Sau khi lưu, hệ thống có thể bổ sung `id`, `doc_type_id`, `assets`, `created_at`, `updated_at`.

### Ví dụ Document

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

## 4. Guard Policy

`Guard Policy` là nhóm quy tắc an toàn áp lên quá trình trả lời.

Mục tiêu:

- Không cho model suy diễn vượt ngoài dữ liệu đã nạp
- Ép hệ thống xử lý rõ các trường hợp không có nguồn hoặc nguồn yếu
- Giảm rủi ro trả lời sai trong ngữ cảnh pháp lý

Danh sách policy hiện được trả về dưới dạng `items`, mỗi item có các trường như:

- `name`
- `enabled`
- `min_retrieved_chunks`
- `min_similarity_score`
- `on_empty_retrieval`
- `on_low_confidence`

### Ví dụ Guard Policy

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

## 5. System Prompt

`System Prompt` là lớp hướng dẫn nền để model biết phải trả lời theo nguyên tắc nào.

Vai trò chính:

- Chỉ bám vào tài liệu đã nạp
- Không tự thêm nội dung ngoài văn bản
- Trả lời rõ ràng khi thiếu dữ liệu

### Ví dụ System Prompt

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

## 6. Ví dụ thực tế

### File nguồn

`52.2014.QH13.docx`

### Trích đoạn mẫu

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

## 7. Tóm tắt áp dụng

Khi áp dụng vào hệ thống, có thể hiểu ngắn gọn như sau:

- `DocType` quyết định cách hiểu và bóc tách loại văn bản
- `Document` đại diện cho file cụ thể được nhập
- `Guard Policy` giới hạn rủi ro trả lời sai
- `System Prompt` định hình cách model phản hồi

Sau khi hoàn tất các cấu hình này, tài liệu mới sẵn sàng đi vào luồng ingest và phục vụ truy xuất trong trải nghiệm chat.
