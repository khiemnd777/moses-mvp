import { ReactNode } from 'react';

type Props = {
  children: ReactNode;
};

const Tag = ({ children }: Props) => <span className="badge">{children}</span>;

export default Tag;
