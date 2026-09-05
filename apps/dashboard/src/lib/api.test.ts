import { afterEach, describe, expect, it, vi } from "vitest";
import { createTemplate } from "#/lib/api";
import type { TemplateInput } from "#/lib/api-types";

const templateInput: TemplateInput = {
    appealable: true,
    context_fields: [],
    description: "Repeated spam",
    levels: [
        {
            actions: [],
            is_default: true,
            name: "Warning",
            notify_user: true,
            position: 1,
            trigger_case_count: 1,
        },
    ],
    name: "Spam",
    reason_template: "Repeated spam",
    slug: "spam",
};

afterEach(() => {
    vi.unstubAllGlobals();
});

describe("dashboard API mutations", () => {
    it("sends a fresh idempotency key when creating a template", async () => {
        const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
            new Response(
                JSON.stringify({
                    template: { id: "template-1" },
                }),
                {
                    headers: { "Content-Type": "application/json" },
                    status: 201,
                },
            ),
        );
        vi.stubGlobal("fetch", fetchMock);

        const template = await createTemplate(
            "guild-1",
            templateInput,
            "csrf-token",
        );

        expect(template.id).toBe("template-1");
        const [, request] = fetchMock.mock.calls[0];
        const headers = new Headers(request?.headers);
        expect(headers.get("Content-Type")).toBe("application/json");
        expect(headers.get("X-CSRF-Token")).toBe("csrf-token");
        expect(headers.get("Idempotency-Key")).toMatch(
            /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i,
        );
    });
});
