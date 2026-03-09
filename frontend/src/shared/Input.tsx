import { InputHTMLAttributes } from 'react';

type Props = InputHTMLAttributes<HTMLInputElement> & {
  label?: string;
};

const Input = ({ label, className = '', ...props }: Props) => {
  return (
    <label className={className}>
      {label && <div className="label">{label}</div>}
      <input className="input" {...props} />
    </label>
  );
};

export default Input;
