import type {
    Appeal,
    AppealList,
    AppealQuestion,
    AppealSettings,
    AppealTransition,
    AuditList,
    Case,
    CaseCreateInput,
    CaseDetail,
    CaseList,
    CaseProfile,
    FailedAction,
    FailedActionList,
    GuildContext,
    GuildListItem,
    GuildSettings,
    GuildSettingsInput,
    MemberCase,
    MemberCaseList,
    Session,
    StaffStatistics,
    Template,
    TemplateInput,
    TemplatePolicy,
} from "#/lib/api-types";

const configuredApiBaseUrl = import.meta.env.VITE_API_BASE_URL?.replace(
    /\/+$/,
    "",
);

export const apiBaseUrl =
    configuredApiBaseUrl ??
    (import.meta.env.DEV ? "http://localhost:8080" : "");

export class ApiError extends Error {
    readonly status: number;
    readonly code?: string;

    constructor(message: string, status: number, code?: string) {
        super(message);
        this.name = "ApiError";
        this.status = status;
        this.code = code;
    }
}

type ApiErrorBody = {
    code?: string;
    error?:
        | string
        | {
              code?: string;
              message?: string;
          };
    message?: string;
};

export function apiUrl(path: string): string {
    return `${apiBaseUrl}${path}`;
}

async function apiRequest<T>(path: string, init: RequestInit = {}): Promise<T> {
    const response = await fetch(apiUrl(path), {
        ...init,
        credentials: "include",
        headers: {
            Accept: "application/json",
            ...init.headers,
        },
    });

    if (!response.ok) {
        let body: ApiErrorBody = {};
        try {
            body = (await response.json()) as ApiErrorBody;
        } catch {
            // Some infrastructure failures do not return the API error envelope.
        }

        const nestedError =
            typeof body.error === "object" ? body.error : undefined;
        const message =
            body.message ??
            nestedError?.message ??
            (typeof body.error === "string" ? body.error : undefined) ??
            `Request failed (${response.status})`;

        throw new ApiError(
            message,
            response.status,
            body.code ?? nestedError?.code,
        );
    }

    if (response.status === 204) {
        return undefined as T;
    }

    return (await response.json()) as T;
}

function guildPath(discordGuildId: string, path = ""): string {
    return `/guilds/${encodeURIComponent(discordGuildId)}${path}`;
}

function idempotentHeaders(
    csrfToken: string,
    contentType?: "application/json",
): Record<string, string> {
    return {
        ...(contentType ? { "Content-Type": contentType } : {}),
        "Idempotency-Key": crypto.randomUUID(),
        "X-CSRF-Token": csrfToken,
    };
}

export function getSession(): Promise<Session> {
    return apiRequest("/auth/me");
}

export async function getMemberCase(caseId: string): Promise<MemberCase> {
    const response = await apiRequest<{ case: MemberCase }>(
        `/members/me/cases/${encodeURIComponent(caseId)}`,
    );
    return response.case;
}

export function getMemberCases(
    guildId: string,
    limit = 50,
    offset = 0,
): Promise<MemberCaseList> {
    const query = new URLSearchParams({
        limit: String(limit),
        offset: String(offset),
    });
    return apiRequest(
        `/members/me/guilds/${encodeURIComponent(guildId)}/cases?${query}`,
    );
}

export async function getMemberAppeal(appealId: string): Promise<Appeal> {
    const response = await apiRequest<{ appeal: Appeal }>(
        `/members/me/appeals/${encodeURIComponent(appealId)}`,
    );
    return response.appeal;
}

export async function submitAppealInformation(
    appealId: string,
    body: string,
    csrfToken: string,
): Promise<Appeal> {
    const response = await apiRequest<{ appeal: Appeal }>(
        `/members/me/appeals/${encodeURIComponent(appealId)}/information`,
        {
            body: JSON.stringify({ body }),
            headers: idempotentHeaders(csrfToken, "application/json"),
            method: "POST",
        },
    );
    return response.appeal;
}

export function logout(csrfToken: string): Promise<void> {
    return apiRequest("/auth/logout", {
        headers: {
            "X-CSRF-Token": csrfToken,
        },
        method: "POST",
    });
}

export async function getGuilds(): Promise<GuildListItem[]> {
    const response = await apiRequest<{ guilds: GuildListItem[] }>("/guilds");
    return response.guilds;
}

export function getGuildContext(discordGuildId: string): Promise<GuildContext> {
    return apiRequest(guildPath(discordGuildId, "/me"));
}

export function getStatistics(
    discordGuildId: string,
): Promise<StaffStatistics> {
    return apiRequest(guildPath(discordGuildId, "/statistics"));
}

export function getCases(
    discordGuildId: string,
    limit = 50,
    offset = 0,
    filters: CaseListFilters = {},
): Promise<CaseList> {
    const query = new URLSearchParams({
        limit: String(limit),
        offset: String(offset),
    });
    if (filters.targetDiscordUserId) {
        query.set("target_discord_user_id", filters.targetDiscordUserId);
    }
    if (filters.templateId) {
        query.set("template_id", filters.templateId);
    }
    if (filters.validity) {
        query.set("validity", filters.validity);
    }
    return apiRequest(guildPath(discordGuildId, `/cases?${query}`));
}

export type CaseListFilters = {
    targetDiscordUserId?: string;
    templateId?: string;
    validity?: string;
};

export function getMemberCaseHistory(
    discordGuildId: string,
    targetDiscordUserId: string,
    limit = 50,
    offset = 0,
): Promise<CaseProfile> {
    const query = new URLSearchParams({
        limit: String(limit),
        offset: String(offset),
    });
    return apiRequest(
        guildPath(
            discordGuildId,
            `/users/${encodeURIComponent(targetDiscordUserId)}/cases?${query}`,
        ),
    );
}

export async function getCase(
    discordGuildId: string,
    caseRef: string,
): Promise<CaseDetail> {
    const response = await apiRequest<{ case: CaseDetail }>(
        guildPath(discordGuildId, `/cases/${encodeURIComponent(caseRef)}`),
    );
    return response.case;
}

export async function createCase(
    discordGuildId: string,
    input: CaseCreateInput,
    csrfToken: string,
): Promise<Case> {
    const response = await apiRequest<{ case: Case }>(
        guildPath(discordGuildId, "/cases"),
        {
            body: JSON.stringify(input),
            headers: idempotentHeaders(csrfToken, "application/json"),
            method: "POST",
        },
    );
    return response.case;
}

export async function voidCase(
    discordGuildId: string,
    caseRef: string,
    reason: string,
    csrfToken: string,
): Promise<Case> {
    const response = await apiRequest<{ case: Case }>(
        guildPath(discordGuildId, `/cases/${encodeURIComponent(caseRef)}/void`),
        {
            body: JSON.stringify({ reason }),
            headers: idempotentHeaders(csrfToken, "application/json"),
            method: "POST",
        },
    );
    return response.case;
}

export function reverseCaseAction(
    discordGuildId: string,
    caseRef: string,
    originalExecutionId: string,
    actionType: string,
    csrfToken: string,
): Promise<unknown> {
    return apiRequest(
        guildPath(
            discordGuildId,
            `/cases/${encodeURIComponent(caseRef)}/reversals`,
        ),
        {
            body: JSON.stringify({
                action_type: actionType,
                confirm: true,
                original_execution_id: originalExecutionId,
            }),
            headers: idempotentHeaders(csrfToken, "application/json"),
            method: "POST",
        },
    );
}

export function getActionFailures(
    discordGuildId: string,
    limit = 50,
    offset = 0,
): Promise<FailedActionList> {
    const query = new URLSearchParams({
        limit: String(limit),
        offset: String(offset),
    });
    return apiRequest(guildPath(discordGuildId, `/action-failures?${query}`));
}

export async function retryActionFailure(
    discordGuildId: string,
    executionId: string,
    csrfToken: string,
): Promise<FailedAction> {
    const response = await apiRequest<{ action: FailedAction }>(
        guildPath(
            discordGuildId,
            `/action-failures/${encodeURIComponent(executionId)}/retry`,
        ),
        {
            headers: idempotentHeaders(csrfToken),
            method: "POST",
        },
    );
    return response.action;
}

export async function dismissActionFailure(
    discordGuildId: string,
    executionId: string,
    csrfToken: string,
): Promise<FailedAction> {
    const response = await apiRequest<{ action: FailedAction }>(
        guildPath(
            discordGuildId,
            `/action-failures/${encodeURIComponent(executionId)}/dismiss`,
        ),
        {
            headers: idempotentHeaders(csrfToken),
            method: "POST",
        },
    );
    return response.action;
}

export async function getTemplates(
    discordGuildId: string,
): Promise<Template[]> {
    const response = await apiRequest<{ templates: Template[] }>(
        guildPath(discordGuildId, "/templates"),
    );
    return response.templates;
}

export async function createTemplate(
    discordGuildId: string,
    input: TemplateInput,
    csrfToken: string,
): Promise<Template> {
    const response = await apiRequest<{ template: Template }>(
        guildPath(discordGuildId, "/templates"),
        {
            body: JSON.stringify(input),
            headers: idempotentHeaders(csrfToken, "application/json"),
            method: "POST",
        },
    );
    return response.template;
}

export async function updateTemplate(
    discordGuildId: string,
    templateId: string,
    input: TemplateInput,
    csrfToken: string,
): Promise<Template> {
    const response = await apiRequest<{ template: Template }>(
        guildPath(
            discordGuildId,
            `/templates/${encodeURIComponent(templateId)}`,
        ),
        {
            body: JSON.stringify(input),
            headers: idempotentHeaders(csrfToken, "application/json"),
            method: "PATCH",
        },
    );
    return response.template;
}

export async function archiveTemplate(
    discordGuildId: string,
    templateId: string,
    csrfToken: string,
): Promise<Template> {
    const response = await apiRequest<{ template: Template }>(
        guildPath(
            discordGuildId,
            `/templates/${encodeURIComponent(templateId)}`,
        ),
        {
            headers: idempotentHeaders(csrfToken),
            method: "DELETE",
        },
    );
    return response.template;
}

export async function restoreTemplate(
    discordGuildId: string,
    templateId: string,
    csrfToken: string,
): Promise<Template> {
    const response = await apiRequest<{ template: Template }>(
        guildPath(
            discordGuildId,
            `/templates/${encodeURIComponent(templateId)}/restore`,
        ),
        {
            headers: idempotentHeaders(csrfToken),
            method: "POST",
        },
    );
    return response.template;
}

export async function exportTemplate(
    discordGuildId: string,
    templateId: string,
): Promise<TemplatePolicy> {
    const response = await apiRequest<{ policy: TemplatePolicy }>(
        guildPath(
            discordGuildId,
            `/templates/${encodeURIComponent(templateId)}/export`,
        ),
    );
    return response.policy;
}

export async function importTemplate(
    discordGuildId: string,
    policy: TemplatePolicy,
    csrfToken: string,
): Promise<Template> {
    const response = await apiRequest<{ template: Template }>(
        guildPath(discordGuildId, "/templates/import"),
        {
            body: JSON.stringify({ confirm: true, policy }),
            headers: idempotentHeaders(csrfToken, "application/json"),
            method: "POST",
        },
    );
    return response.template;
}

export function getAppeals(
    discordGuildId: string,
    limit = 50,
    offset = 0,
    status?: string,
): Promise<AppealList> {
    const query = new URLSearchParams({
        limit: String(limit),
        offset: String(offset),
    });
    if (status) {
        query.set("status", status);
    }
    return apiRequest(guildPath(discordGuildId, `/appeals?${query}`));
}

export function getAppealSettings(
    discordGuildId: string,
): Promise<AppealSettings> {
    return apiRequest(guildPath(discordGuildId, "/appeal-settings"));
}

export function updateAppealSettings(
    discordGuildId: string,
    questions: AppealQuestion[],
    csrfToken: string,
): Promise<AppealSettings> {
    return apiRequest(guildPath(discordGuildId, "/appeal-settings"), {
        body: JSON.stringify({ questions }),
        headers: idempotentHeaders(csrfToken, "application/json"),
        method: "PUT",
    });
}

export async function transitionAppeal(
    discordGuildId: string,
    appealId: string,
    transition: AppealTransition,
    reason: string,
    csrfToken: string,
): Promise<Appeal> {
    const response = await apiRequest<{ appeal: Appeal }>(
        guildPath(
            discordGuildId,
            `/appeals/${encodeURIComponent(appealId)}/${transition}`,
        ),
        {
            body: JSON.stringify({ reason }),
            headers: idempotentHeaders(csrfToken, "application/json"),
            method: "POST",
        },
    );
    return response.appeal;
}

export function reverseAppealAction(
    discordGuildId: string,
    appealId: string,
    originalExecutionId: string,
    actionType: string,
    csrfToken: string,
): Promise<unknown> {
    return apiRequest(
        guildPath(
            discordGuildId,
            `/appeals/${encodeURIComponent(appealId)}/reversals`,
        ),
        {
            body: JSON.stringify({
                action_type: actionType,
                confirm: true,
                original_execution_id: originalExecutionId,
            }),
            headers: idempotentHeaders(csrfToken, "application/json"),
            method: "POST",
        },
    );
}

export function getAuditLog(
    discordGuildId: string,
    limit = 50,
    offset = 0,
    filters: AuditLogFilters = {},
): Promise<AuditList> {
    const query = new URLSearchParams({
        limit: String(limit),
        offset: String(offset),
    });
    if (filters.action) {
        query.set("action", filters.action);
    }
    if (filters.resourceType) {
        query.set("resource_type", filters.resourceType);
    }
    if (filters.result) {
        query.set("result", filters.result);
    }
    return apiRequest(guildPath(discordGuildId, `/audit-log?${query}`));
}

export type AuditLogFilters = {
    action?: string;
    resourceType?: string;
    result?: string;
};

export async function getGuildSettings(
    discordGuildId: string,
): Promise<GuildSettings> {
    const response = await apiRequest<{ settings: GuildSettings }>(
        guildPath(discordGuildId, "/settings"),
    );
    return response.settings;
}

export async function acknowledgeStarterPolicyNotice(
    discordGuildId: string,
    csrfToken: string,
): Promise<GuildSettings> {
    const response = await apiRequest<{ settings: GuildSettings }>(
        guildPath(
            discordGuildId,
            "/settings/starter-policy-notice/acknowledge",
        ),
        {
            headers: idempotentHeaders(csrfToken),
            method: "POST",
        },
    );
    return response.settings;
}

export async function updateGuildSettings(
    discordGuildId: string,
    input: GuildSettingsInput,
    csrfToken: string,
): Promise<GuildSettings> {
    const response = await apiRequest<{ settings: GuildSettings }>(
        guildPath(discordGuildId, "/settings"),
        {
            body: JSON.stringify(input),
            headers: idempotentHeaders(csrfToken, "application/json"),
            method: "PATCH",
        },
    );
    return response.settings;
}
