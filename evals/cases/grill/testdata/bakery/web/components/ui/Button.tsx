export function Button({ children, onClick }: { children: React.ReactNode; onClick?: () => void }) {
  return <button className="crumb-button" onClick={onClick}>{children}</button>;
}
