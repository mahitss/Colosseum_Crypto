import { NextRequest, NextResponse } from "next/server";
import { updateAlertRule, deleteAlertRule } from "@/lib/enterprise-api";
import type { AlertRuleInput } from "@/lib/api-types";

interface RouteContext {
  params: Promise<{ id: string }>;
}

export async function PATCH(request: NextRequest, context: RouteContext) {
  try {
    const { id } = await context.params;
    const body = (await request.json()) as Partial<AlertRuleInput>;

    if (body.name !== undefined && !body.name.trim()) {
      return NextResponse.json({ error: { message: "Name cannot be empty" } }, { status: 400 });
    }

    const rule = await updateAlertRule(id, body as AlertRuleInput);
    return NextResponse.json(rule);
  } catch (error) {
    console.error("Failed to update alert rule:", error);
    return NextResponse.json({ error: "Failed to update alert rule" }, { status: 500 });
  }
}

export async function DELETE(_request: NextRequest, context: RouteContext) {
  try {
    const { id } = await context.params;
    await deleteAlertRule(id);
    return new NextResponse(null, { status: 204 });
  } catch (error) {
    console.error("Failed to delete alert rule:", error);
    return NextResponse.json({ error: "Failed to delete alert rule" }, { status: 500 });
  }
}
