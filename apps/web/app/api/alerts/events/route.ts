import { NextRequest, NextResponse } from "next/server";
import { getAlertEvents } from "@/lib/enterprise-api";

const MAX_LIMIT = 100;

export async function GET(request: NextRequest) {
  try {
    const raw = request.nextUrl.searchParams.get("limit");
    let limit = 25;
    if (raw) {
      const parsed = Number.parseInt(raw, 10);
      if (!Number.isFinite(parsed) || parsed < 1) {
        return NextResponse.json({ error: "limit must be a positive integer" }, { status: 400 });
      }
      // Bounded on both sides: a caller cannot ask for an unbounded history,
      // and a typo cannot ask for ten thousand rows.
      limit = Math.min(parsed, MAX_LIMIT);
    }
    const events = await getAlertEvents(limit);
    return NextResponse.json(events);
  } catch (error) {
    console.error("Failed to fetch alert events:", error);
    return NextResponse.json({ error: "Failed to fetch alert events" }, { status: 500 });
  }
}
