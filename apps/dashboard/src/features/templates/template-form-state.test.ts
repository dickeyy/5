import { describe, expect, it } from "vitest";
import type { Template } from "#/lib/api-types";
import {
    createTemplateDraft,
    type TemplateDraft,
    templateInputFromDraft,
} from "./template-form-state";

describe("template form state", () => {
    it("starts a new policy with one non-enforcing default level", () => {
        const draft = createTemplateDraft();

        expect(draft.appealable).toBe(true);
        expect(draft.levels).toMatchObject([
            {
                actionType: "none",
                isDefault: true,
                notifyUser: true,
                triggerCaseCount: 0,
            },
        ]);
    });

    it("sorts persisted levels before presenting them for editing", () => {
        const template: Template = {
            appealable: false,
            archived_at: null,
            context_fields: [],
            description: "",
            guild_id: "guild",
            id: "template",
            levels: [
                {
                    actions: [],
                    id: "second",
                    is_default: false,
                    name: "Second",
                    notify_user: false,
                    position: 2,
                    trigger_case_count: 2,
                },
                {
                    actions: [],
                    id: "default",
                    is_default: true,
                    name: "Default",
                    notify_user: true,
                    position: 1,
                    trigger_case_count: 0,
                },
            ],
            name: "Policy",
            reason_template: "Reason",
            slug: "policy",
            version: 1,
        };

        expect(
            createTemplateDraft(template).levels.map((level) => level.id),
        ).toEqual(["default", "second"]);
    });

    it("normalizes form values into the API contract", () => {
        const draft: TemplateDraft = {
            appealable: true,
            contextFields: [
                {
                    id: "context",
                    key: " MESSAGE ",
                    label: " Message ",
                    required: true,
                    type: "discord_message_link",
                },
            ],
            description: " Description ",
            levels: [
                {
                    actionType: "timeout_user",
                    deleteMessageSeconds: 0,
                    id: "default",
                    isDefault: true,
                    maxRetries: 2,
                    name: " Default ",
                    notifyUser: true,
                    timeoutDurationSeconds: 3600,
                    triggerCaseCount: 5,
                },
            ],
            name: " Policy ",
            reasonTemplate: " Official reason ",
            slug: " POLICY ",
        };

        expect(templateInputFromDraft(draft)).toEqual({
            appealable: true,
            context_fields: [
                {
                    key: "message",
                    label: "Message",
                    position: 1,
                    required: true,
                    type: "discord_message_link",
                },
            ],
            description: "Description",
            levels: [
                {
                    actions: [
                        {
                            action_type: "timeout_user",
                            delete_message_seconds: undefined,
                            max_retries: 2,
                            timeout_duration_seconds: 3600,
                        },
                    ],
                    is_default: true,
                    name: "Default",
                    notify_user: true,
                    position: 1,
                    trigger_case_count: 0,
                },
            ],
            name: "Policy",
            reason_template: "Official reason",
            slug: "policy",
        });
    });
});
