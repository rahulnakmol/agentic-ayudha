export async function POST(req: Request) {
  const order = await req.json();
  return Response.json({ id: "ord_0001", ...order }, { status: 201 });
}
