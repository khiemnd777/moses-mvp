import { ReactNode } from 'react';

type Props = {
  title?: ReactNode;
  children: ReactNode;
  className?: string;
};

const Panel = ({ title, children, className = '' }: Props) => {
  return (
    <section className={`card ${className}`.trim()}>
      {title && <header style={{ padding: '16px 18px', borderBottom: '1px solid var(--border)' }}>{title}</header>}
      <div style={{ padding: '16px 18px' }}>{children}</div>
    </section>
  );
};

export default Panel;
