import { searchProducts } from "../../../../lib/search";

export async function GET(req: Request) {
  const q = new URL(req.url).searchParams.get("q") ?? "";
  return Response.json(searchProducts(q));
}
