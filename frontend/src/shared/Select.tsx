import { SelectHTMLAttributes } from 'react';

type Props = SelectHTMLAttributes<HTMLSelectElement> & {
  label?: string;
};

const Select = ({ label, className = '', children, ...props }: Props) => {
  return (
    <label className={className}>
      {label && <div className="label">{label}</div>}
      <select className="select" {...props}>
        {children}
      </select>
    </label>
  );
};

export default Select;
