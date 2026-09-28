import { NextRequest, NextResponse } from "next/server";
import { deleteWatchlist } from "@/lib/enterprise-api";

export async function DELETE(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = await params;
    await deleteWatchlist(id);
    return NextResponse.json({ success: true });
  } catch (error) {
    console.error("Failed to delete watchlist:", error);
    return NextResponse.json({ error: "Failed to delete watchlist" }, { status: 500 });
  }
}