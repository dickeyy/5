import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { PageHeader } from "#/components/dashboard/page-header";
import { QueryError, QueryPending } from "#/components/dashboard/query-state";
import { Button } from "#/components/ui/button";
import { Field, FieldLabel } from "#/components/ui/field";
import { Input } from "#/components/ui/input";
import {
    Select,
    SelectItem,
    SelectPopup,
    SelectTrigger,
    SelectValue,
} from "#/components/ui/select";
import { CaseTable } from "#/features/cases/case-table";
import { CreateCaseDialog } from "#/features/cases/create-case-dialog";
import type { CaseListFilters } from "#/lib/api";
import {
    casesQuery,
    guildContextQuery,
    sessionQuery,
    templatesQuery,
} from "#/lib/queries";

export const Route = createFileRoute("/guilds/$guildId/cases")({
    component: CasesPage,
});

function CasesPage() {
    const { guildId } = Route.useParams();
    const [filters, setFilters] = useState<CaseListFilters>({});
    const [offset, setOffset] = useState(0);
    const pageSize = 25;
    const cases = useQuery(casesQuery(guildId, pageSize, offset, filters));
    const guildContext = useQuery(guildContextQuery(guildId));
    const session = useQuery(sessionQuery);
    const templates = useQuery(templatesQuery(guildId));
    const canCreateCase = Boolean(
        guildContext.data?.permissions["case.create"],
    );

    return (
        <>
            <PageHeader
                actions={
                    canCreateCase && session.data && templates.data ? (
                        <CreateCaseDialog
                            csrfToken={session.data.csrf_token}
                            guildId={guildId}
                            templates={templates.data}
                        />
                    ) : undefined
                }
                description="Filterable moderation history for this server."
                title="Cases"
            />
            <form
                className="grid items-end gap-3 rounded-xl border p-4 sm:grid-cols-2 lg:grid-cols-[minmax(12rem,1fr)_minmax(12rem,1fr)_12rem_auto]"
                onSubmit={(event) => {
                    event.preventDefault();
                    const formData = new FormData(event.currentTarget);
                    const templateId = String(
                        formData.get("template_id") ?? "",
                    );
                    const validity = String(formData.get("validity") ?? "");
                    setFilters({
                        targetDiscordUserId:
                            String(
                                formData.get("target_discord_user_id") ?? "",
                            ).trim() || undefined,
                        templateId:
                            templateId === "all" ? undefined : templateId,
                        validity: validity === "all" ? undefined : validity,
                    });
                    setOffset(0);
                }}
            >
                <Field name="target_discord_user_id">
                    <FieldLabel>Member ID</FieldLabel>
                    <Input
                        inputMode="numeric"
                        name="target_discord_user_id"
                        pattern="[0-9]*"
                        placeholder="All members"
                    />
                </Field>
                <Field name="template_id">
                    <FieldLabel>Template</FieldLabel>
                    <Select
                        defaultValue="all"
                        items={[
                            { label: "All templates", value: "all" },
                            ...(templates.data ?? []).map((template) => ({
                                label: template.name,
                                value: template.id,
                            })),
                        ]}
                        name="template_id"
                    >
                        <SelectTrigger>
                            <SelectValue />
                        </SelectTrigger>
                        <SelectPopup>
                            <SelectItem value="all">All templates</SelectItem>
                            {templates.data?.map((template) => (
                                <SelectItem
                                    key={template.id}
                                    value={template.id}
                                >
                                    {template.name}
                                </SelectItem>
                            ))}
                        </SelectPopup>
                    </Select>
                </Field>
                <Field name="validity">
                    <FieldLabel>Validity</FieldLabel>
                    <Select
                        defaultValue="all"
                        items={[
                            { label: "All cases", value: "all" },
                            { label: "Valid", value: "valid" },
                            { label: "Voided", value: "voided" },
                        ]}
                        name="validity"
                    >
                        <SelectTrigger>
                            <SelectValue />
                        </SelectTrigger>
                        <SelectPopup>
                            <SelectItem value="all">All cases</SelectItem>
                            <SelectItem value="valid">Valid</SelectItem>
                            <SelectItem value="voided">Voided</SelectItem>
                        </SelectPopup>
                    </Select>
                </Field>
                <Button type="submit" variant="outline">
                    Apply filters
                </Button>
            </form>
            {cases.isPending ? (
                <QueryPending label="Loading cases" />
            ) : cases.isError ? (
                <QueryError onRetry={() => cases.refetch()} />
            ) : (
                <>
                    <CaseTable cases={cases.data.cases} guildId={guildId} />
                    <div className="flex items-center justify-between gap-3">
                        <p className="text-muted-foreground text-sm">
                            {cases.data.total === 0
                                ? "No results"
                                : `${offset + 1}–${Math.min(
                                      offset + cases.data.cases.length,
                                      cases.data.total,
                                  )} of ${cases.data.total}`}
                        </p>
                        <div className="flex gap-2">
                            <Button
                                disabled={offset === 0}
                                onClick={() =>
                                    setOffset((current) =>
                                        Math.max(0, current - pageSize),
                                    )
                                }
                                size="sm"
                                type="button"
                                variant="outline"
                            >
                                Previous
                            </Button>
                            <Button
                                disabled={
                                    offset + cases.data.cases.length >=
                                    cases.data.total
                                }
                                onClick={() =>
                                    setOffset((current) => current + pageSize)
                                }
                                size="sm"
                                type="button"
                                variant="outline"
                            >
                                Next
                            </Button>
                        </div>
                    </div>
                </>
            )}
        </>
    );
}
