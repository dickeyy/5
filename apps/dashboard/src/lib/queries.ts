import { queryOptions } from "@tanstack/react-query";
import type { AuditLogFilters, CaseListFilters } from "#/lib/api";
import {
    getActionFailures,
    getAppealSettings,
    getAppeals,
    getAuditLog,
    getCase,
    getCases,
    getGuildContext,
    getGuildSettings,
    getGuilds,
    getMemberAppeal,
    getMemberCase,
    getMemberCaseHistory,
    getMemberCases,
    getSession,
    getStatistics,
    getTemplates,
} from "#/lib/api";

export const sessionQuery = queryOptions({
    queryKey: ["session"],
    queryFn: getSession,
    retry: false,
    staleTime: 60_000,
});

export const guildsQuery = queryOptions({
    queryKey: ["guilds"],
    queryFn: getGuilds,
    staleTime: 60_000,
});

export function memberCaseQuery(caseId: string) {
    return queryOptions({
        queryKey: ["member", "case", caseId],
        queryFn: () => getMemberCase(caseId),
    });
}

export function memberCasesQuery(guildId: string, limit = 50, offset = 0) {
    return queryOptions({
        queryKey: ["member", "guild", guildId, "cases", { limit, offset }],
        queryFn: () => getMemberCases(guildId, limit, offset),
    });
}

export function memberAppealQuery(appealId: string) {
    return queryOptions({
        queryKey: ["member", "appeal", appealId],
        queryFn: () => getMemberAppeal(appealId),
    });
}

export function guildContextQuery(discordGuildId: string) {
    return queryOptions({
        queryKey: ["guild", discordGuildId, "context"],
        queryFn: () => getGuildContext(discordGuildId),
        staleTime: 30_000,
    });
}

export function statisticsQuery(discordGuildId: string) {
    return queryOptions({
        queryKey: ["guild", discordGuildId, "statistics"],
        queryFn: () => getStatistics(discordGuildId),
        staleTime: 30_000,
    });
}

export function casesQuery(
    discordGuildId: string,
    limit = 50,
    offset = 0,
    filters: CaseListFilters = {},
) {
    return queryOptions({
        queryKey: [
            "guild",
            discordGuildId,
            "cases",
            { filters, limit, offset },
        ],
        queryFn: () => getCases(discordGuildId, limit, offset, filters),
    });
}

export function caseQuery(discordGuildId: string, caseRef: string) {
    return queryOptions({
        queryKey: ["guild", discordGuildId, "case", caseRef],
        queryFn: () => getCase(discordGuildId, caseRef),
    });
}

export function memberCaseHistoryQuery(
    discordGuildId: string,
    targetDiscordUserId: string,
    limit = 50,
    offset = 0,
) {
    return queryOptions({
        queryKey: [
            "guild",
            discordGuildId,
            "member",
            targetDiscordUserId,
            "cases",
            { limit, offset },
        ],
        queryFn: () =>
            getMemberCaseHistory(
                discordGuildId,
                targetDiscordUserId,
                limit,
                offset,
            ),
    });
}

export function templatesQuery(discordGuildId: string) {
    return queryOptions({
        queryKey: ["guild", discordGuildId, "templates"],
        queryFn: () => getTemplates(discordGuildId),
    });
}

export function appealsQuery(
    discordGuildId: string,
    limit = 50,
    offset = 0,
    status?: string,
) {
    return queryOptions({
        queryKey: [
            "guild",
            discordGuildId,
            "appeals",
            { limit, offset, status },
        ],
        queryFn: () => getAppeals(discordGuildId, limit, offset, status),
    });
}

export function appealSettingsQuery(discordGuildId: string) {
    return queryOptions({
        queryKey: ["guild", discordGuildId, "appeal-settings"],
        queryFn: () => getAppealSettings(discordGuildId),
    });
}

export function actionFailuresQuery(
    discordGuildId: string,
    limit = 50,
    offset = 0,
) {
    return queryOptions({
        queryKey: [
            "guild",
            discordGuildId,
            "action-failures",
            { limit, offset },
        ],
        queryFn: () => getActionFailures(discordGuildId, limit, offset),
    });
}

export function auditLogQuery(
    discordGuildId: string,
    limit = 50,
    offset = 0,
    filters: AuditLogFilters = {},
) {
    return queryOptions({
        queryKey: [
            "guild",
            discordGuildId,
            "audit-log",
            { filters, limit, offset },
        ],
        queryFn: () => getAuditLog(discordGuildId, limit, offset, filters),
    });
}

export function guildSettingsQuery(discordGuildId: string) {
    return queryOptions({
        queryKey: ["guild", discordGuildId, "settings"],
        queryFn: () => getGuildSettings(discordGuildId),
    });
}
