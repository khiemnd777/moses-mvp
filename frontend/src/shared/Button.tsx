import { ButtonHTMLAttributes } from 'react';

type Props = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: 'primary' | 'secondary' | 'outline';
};

const Button = ({ variant = 'primary', className = '', ...props }: Props) => {
  const variantClass =
    variant === 'secondary' ? 'secondary' : variant === 'outline' ? 'outline' : '';
  return <button className={`button ${variantClass} ${className}`.trim()} {...props} />;
};

export default Button;
