import { ReactNode } from 'react';

type Props = {
  title?: ReactNode;
  children: ReactNode;
  className?: string;
  headerClassName?: string;
  bodyClassName?: string;
};

const Panel = ({ title, children, className = '', headerClassName = '', bodyClassName = '' }: Props) => {
  return (
    <section className={`card ${className}`.trim()}>
      {title && <header className={`panel-header ${headerClassName}`.trim()}>{title}</header>}
      <div className={`panel-body ${bodyClassName}`.trim()}>{children}</div>
    </section>
  );
};

export default Panel;
