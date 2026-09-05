import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { ArchiveIcon, BellIcon, BellOffIcon } from "lucide-react";
import { PageHeader } from "#/components/dashboard/page-header";
import { QueryError, QueryPending } from "#/components/dashboard/query-state";
import { Badge } from "#/components/ui/badge";
import {
    Card,
    CardDescription,
    CardFooter,
    CardHeader,
    CardPanel,
    CardTitle,
} from "#/components/ui/card";
import {
    Empty,
    EmptyDescription,
    EmptyHeader,
    EmptyMedia,
    EmptyTitle,
} from "#/components/ui/empty";
import { TemplateAvailabilityActions } from "#/features/templates/template-availability-actions";
import { TemplateFormDialog } from "#/features/templates/template-form-dialog";
import {
    ExportTemplateButton,
    ImportTemplateDialog,
} from "#/features/templates/template-policy-actions";
import { titleCase } from "#/lib/format";
import { guildContextQuery, sessionQuery, templatesQuery } from "#/lib/queries";

export const Route = createFileRoute("/guilds/$guildId/templates")({
    component: TemplatesPage,
});

function TemplatesPage() {
    const { guildId } = Route.useParams();
    const templates = useQuery(templatesQuery(guildId));
    const guildContext = useQuery(guildContextQuery(guildId));
    const session = useQuery(sessionQuery);
    const canWrite = Boolean(
        guildContext.data?.permissions["case_template.write"],
    );
    const canArchive = Boolean(
        guildContext.data?.permissions["case_template.delete"],
    );

    return (
        <>
            <PageHeader
                actions={
                    canWrite && session.data ? (
                        <>
                            <ImportTemplateDialog
                                csrfToken={session.data.csrf_token}
                                guildId={guildId}
                            />
                            <TemplateFormDialog
                                csrfToken={session.data.csrf_token}
                                guildId={guildId}
                            />
                        </>
                    ) : undefined
                }
                description="Rules, context requirements, and automatic escalation."
                title="Templates"
            />
            {templates.isPending ? (
                <QueryPending label="Loading templates" />
            ) : templates.isError ? (
                <QueryError onRetry={() => templates.refetch()} />
            ) : templates.data.length === 0 ? (
                <Empty className="rounded-2xl border">
                    <EmptyMedia variant="icon">
                        <ArchiveIcon aria-hidden="true" />
                    </EmptyMedia>
                    <EmptyHeader>
                        <EmptyTitle>No templates</EmptyTitle>
                        <EmptyDescription>
                            This server does not have any moderation templates.
                        </EmptyDescription>
                    </EmptyHeader>
                </Empty>
            ) : (
                <section
                    aria-label="Case templates"
                    className="grid gap-4 lg:grid-cols-2"
                >
                    {templates.data.map((template) => (
                        <Card key={template.id}>
                            <CardHeader>
                                <div className="flex items-center gap-2">
                                    <CardTitle>{template.name}</CardTitle>
                                    {template.archived_at ? (
                                        <Badge variant="secondary">
                                            Archived
                                        </Badge>
                                    ) : (
                                        <Badge variant="success">Active</Badge>
                                    )}
                                </div>
                                <CardDescription>
                                    {template.description ||
                                        template.reason_template}
                                </CardDescription>
                            </CardHeader>
                            <CardPanel className="flex flex-col gap-4">
                                <div className="flex flex-wrap gap-2 text-muted-foreground text-xs">
                                    <span>Version {template.version}</span>
                                    <span aria-hidden="true">·</span>
                                    <span>
                                        {template.context_fields.length} context
                                        fields
                                    </span>
                                    <span aria-hidden="true">·</span>
                                    <span>
                                        {template.appealable
                                            ? "Appealable"
                                            : "Not appealable"}
                                    </span>
                                </div>
                                <div className="flex flex-col gap-2">
                                    {template.levels
                                        .slice()
                                        .sort(
                                            (left, right) =>
                                                left.position - right.position,
                                        )
                                        .map((level) => (
                                            <div
                                                className="flex items-center justify-between gap-4 rounded-lg border px-3 py-2"
                                                key={level.id}
                                            >
                                                <div className="min-w-0">
                                                    <p className="truncate font-medium text-sm">
                                                        {level.name}
                                                    </p>
                                                    <p className="text-muted-foreground text-xs">
                                                        {level.is_default
                                                            ? "Default"
                                                            : `Starts at case ${level.trigger_case_count}`}
                                                    </p>
                                                </div>
                                                <div className="flex items-center gap-2 text-muted-foreground text-xs">
                                                    {level.notify_user ? (
                                                        <BellIcon aria-label="Notifies member" />
                                                    ) : (
                                                        <BellOffIcon aria-label="Does not notify member" />
                                                    )}
                                                    <span>
                                                        {titleCase(
                                                            level.actions[0]
                                                                ?.action_type ??
                                                                "Case only",
                                                        )}
                                                    </span>
                                                </div>
                                            </div>
                                        ))}
                                </div>
                            </CardPanel>
                            {session.data && (canWrite || canArchive) ? (
                                <CardFooter className="flex-wrap justify-end gap-2">
                                    {canWrite ? (
                                        <>
                                            <ExportTemplateButton
                                                guildId={guildId}
                                                template={template}
                                            />
                                            <TemplateFormDialog
                                                csrfToken={
                                                    session.data.csrf_token
                                                }
                                                guildId={guildId}
                                                template={template}
                                            />
                                        </>
                                    ) : null}
                                    {canArchive ? (
                                        <TemplateAvailabilityActions
                                            csrfToken={session.data.csrf_token}
                                            guildId={guildId}
                                            template={template}
                                        />
                                    ) : null}
                                </CardFooter>
                            ) : null}
                        </Card>
                    ))}
                </section>
            )}
        </>
    );
}
