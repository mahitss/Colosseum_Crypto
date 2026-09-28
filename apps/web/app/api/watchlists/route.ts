import { NextRequest, NextResponse } from "next/server";
import { getWatchlists, createWatchlist } from "@/lib/enterprise-api";

export async function GET() {
  try {
    const watchlists = await getWatchlists();
    return NextResponse.json(watchlists);
  } catch (error) {
    console.error("Failed to fetch watchlists:", error);
    return NextResponse.json({ error: "Failed to fetch watchlists" }, { status: 500 });
  }
}

export async function POST(request: NextRequest) {
  try {
    const body = await request.json();
    const { name, description } = body;

    if (!name || !name.trim()) {
      return NextResponse.json({ error: "Name is required" }, { status: 400 });
    }

    const watchlist = await createWatchlist({ name: name.trim(), description: description?.trim() || null });
    return NextResponse.json(watchlist, { status: 201 });
  } catch (error) {
    console.error("Failed to create watchlist:", error);
    return NextResponse.json({ error: "Failed to create watchlist" }, { status: 500 });
  }
}
