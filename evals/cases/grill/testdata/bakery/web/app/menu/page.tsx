import { ProductTile } from "../../components/ProductTile";
import { track } from "../../../lib/analytics";

export default function Menu() {
  track("menu_viewed");
  return (
    <main>
      <h1>Menu</h1>
      <ProductTile name="Sourdough" price="5.00" />
    </main>
  );
}
