import { NextRequest, NextResponse } from "next/server";
import { getAlertRules, createAlertRule } from "@/lib/enterprise-api";
import type { AlertRuleInput } from "@/lib/api-types";

export async function GET() {
  try {
    const rules = await getAlertRules();
    return NextResponse.json(rules);
  } catch (error) {
    console.error("Failed to fetch alert rules:", error);
    return NextResponse.json({ error: "Failed to fetch alert rules" }, { status: 500 });
  }
}

export async function POST(request: NextRequest) {
  try {
    const body = (await request.json()) as Partial<AlertRuleInput>;

    // The gateway owns validation; this is a fast local rejection so an
    // obviously-empty form does not cost a round trip. The gateway remains the
    // authority and its error message is surfaced verbatim to the user.
    if (!body.name || !body.name.trim()) {
      return NextResponse.json({ error: { message: "Name is required" } }, { status: 400 });
    }

    const rule = await createAlertRule({ ...body, name: body.name.trim() });
    return NextResponse.json(rule, { status: 201 });
  } catch (error) {
    console.error("Failed to create alert rule:", error);
    return NextResponse.json({ error: "Failed to create alert rule" }, { status: 500 });
  }
}
