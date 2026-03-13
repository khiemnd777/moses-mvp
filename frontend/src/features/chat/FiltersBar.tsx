import Input from '@/shared/Input';
import Select from '@/shared/Select';
import { useChatStore } from './chatStore';

const FiltersBar = () => {
  const { filters, updateFilters } = useChatStore();

  return (
    <div className="grid two">
      <Select label="Tone" value={filters.tone} onChange={(e) => updateFilters({ tone: e.target.value })}>
        <option value="default">Mặc đinh</option>
        <option value="academic">Hàn lâm</option>
        <option value="procedure">Thủ tục</option>
      </Select>
      <Input
        label="Top K"
        type="number"
        min={1}
        max={20}
        value={filters.topK}
        onChange={(e) => updateFilters({ topK: Number(e.target.value) })}
      />
      <Select
        label="Effective Status"
        value={filters.effectiveStatus}
        onChange={(e) => updateFilters({ effectiveStatus: e.target.value })}
      >
        <option value="active">Còn hiệu lực</option>
        <option value="archived">Lưu trữ</option>
      </Select>
      <Input
        label="Domain"
        placeholder="hon_nhan_gia_dinh"
        value={filters.domain}
        onChange={(e) => updateFilters({ domain: e.target.value })}
      />
      <Input
        label="Doc Type"
        placeholder="law"
        value={filters.docType}
        onChange={(e) => updateFilters({ docType: e.target.value })}
      />
      <Input
        label="Số văn bản"
        placeholder="56/2014/QH13"
        value={filters.documentNumber || ''}
        onChange={(e) => updateFilters({ documentNumber: e.target.value })}
      />
      <Input
        label="Điều"
        placeholder="56"
        value={filters.articleNumber || ''}
        onChange={(e) => updateFilters({ articleNumber: e.target.value })}
      />
    </div>
  );
};

export default FiltersBar;
