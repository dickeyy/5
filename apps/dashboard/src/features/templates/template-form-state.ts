import type { Template, TemplateInput } from "#/lib/api-types";

export type ContextFieldDraft = {
    id: string;
    key: string;
    label: string;
    type: string;
    required: boolean;
};

export type LevelDraft = {
    id: string;
    name: string;
    isDefault: boolean;
    triggerCaseCount: number;
    notifyUser: boolean;
    actionType: string;
    timeoutDurationSeconds: number;
    deleteMessageSeconds: number;
    maxRetries: number;
};

export type TemplateDraft = {
    slug: string;
    name: string;
    description: string;
    reasonTemplate: string;
    appealable: boolean;
    contextFields: ContextFieldDraft[];
    levels: LevelDraft[];
};

export function createTemplateDraft(template?: Template): TemplateDraft {
    if (!template) {
        return {
            appealable: true,
            contextFields: [],
            description: "",
            levels: [createLevelDraft(true)],
            name: "",
            reasonTemplate: "",
            slug: "",
        };
    }

    return {
        appealable: template.appealable,
        contextFields: template.context_fields.map((field) => ({
            id: field.id,
            key: field.key,
            label: field.label,
            required: field.required,
            type: field.type,
        })),
        description: template.description,
        levels: template.levels
            .slice()
            .sort((left, right) => left.position - right.position)
            .map((level) => ({
                actionType: level.actions[0]?.action_type ?? "none",
                deleteMessageSeconds:
                    level.actions[0]?.delete_message_seconds ?? 0,
                id: level.id,
                isDefault: level.is_default,
                maxRetries: level.actions[0]?.max_retries ?? 0,
                name: level.name,
                notifyUser: level.notify_user,
                timeoutDurationSeconds:
                    level.actions[0]?.timeout_duration_seconds ?? 86_400,
                triggerCaseCount: level.trigger_case_count,
            })),
        name: template.name,
        reasonTemplate: template.reason_template,
        slug: template.slug,
    };
}

export function createContextFieldDraft(): ContextFieldDraft {
    return {
        id: crypto.randomUUID(),
        key: "",
        label: "",
        required: true,
        type: "short_text",
    };
}

export function createLevelDraft(isDefault = false): LevelDraft {
    return {
        actionType: "none",
        deleteMessageSeconds: 0,
        id: crypto.randomUUID(),
        isDefault,
        maxRetries: 3,
        name: isDefault ? "Default" : "",
        notifyUser: true,
        timeoutDurationSeconds: 86_400,
        triggerCaseCount: isDefault ? 0 : 2,
    };
}

export function templateInputFromDraft(draft: TemplateDraft): TemplateInput {
    return {
        appealable: draft.appealable,
        context_fields: draft.contextFields.map((field, index) => ({
            key: field.key.trim().toLowerCase(),
            label: field.label.trim(),
            position: index + 1,
            required: field.required,
            type: field.type,
        })),
        description: draft.description.trim(),
        levels: draft.levels.map((level, index) => ({
            actions:
                level.actionType === "none"
                    ? []
                    : [
                          {
                              action_type: level.actionType,
                              delete_message_seconds:
                                  level.actionType === "ban_user"
                                      ? level.deleteMessageSeconds
                                      : undefined,
                              max_retries: level.maxRetries,
                              timeout_duration_seconds:
                                  level.actionType === "timeout_user"
                                      ? level.timeoutDurationSeconds
                                      : undefined,
                          },
                      ],
            is_default: level.isDefault,
            name: level.name.trim(),
            notify_user: level.notifyUser,
            position: index + 1,
            trigger_case_count: level.isDefault ? 0 : level.triggerCaseCount,
        })),
        name: draft.name.trim(),
        reason_template: draft.reasonTemplate.trim(),
        slug: draft.slug.trim().toLowerCase(),
    };
}
