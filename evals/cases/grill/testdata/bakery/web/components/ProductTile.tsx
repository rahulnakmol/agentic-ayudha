import { Button } from "./ui/Button";

export function ProductTile({ name, price }: { name: string; price: string }) {
  return (
    <div className="crumb-tile">
      <h3>{name}</h3>
      <p>{price}</p>
      <Button>Add</Button>
    </div>
  );
}
