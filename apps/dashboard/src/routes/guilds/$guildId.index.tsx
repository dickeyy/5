import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { PageHeader } from "#/components/dashboard/page-header";
import { QueryError, QueryPending } from "#/components/dashboard/query-state";
import { Alert, AlertDescription, AlertTitle } from "#/components/ui/alert";
import { Card, CardHeader, CardTitle } from "#/components/ui/card";
import { CaseTable } from "#/features/cases/case-table";
import { casesQuery, guildContextQuery, statisticsQuery } from "#/lib/queries";

export const Route = createFileRoute("/guilds/$guildId/")({
    component: Overview,
});

function Overview() {
    const { guildId } = Route.useParams();
    const guildContext = useQuery(guildContextQuery(guildId));
    const canReadAudit = Boolean(guildContext.data?.permissions["audit.read"]);
    const canReadCases = Boolean(guildContext.data?.permissions["case.read"]);
    const statistics = useQuery({
        ...statisticsQuery(guildId),
        enabled: canReadAudit,
    });
    const cases = useQuery({
        ...casesQuery(guildId, 8),
        enabled: canReadCases,
    });

    if (
        guildContext.isPending ||
        (canReadAudit && statistics.isPending) ||
        (canReadCases && cases.isPending)
    ) {
        return <QueryPending label="Loading overview" />;
    }

    if (
        guildContext.isError ||
        (canReadAudit && statistics.isError) ||
        (canReadCases && cases.isError)
    ) {
        return (
            <QueryError
                onRetry={() => {
                    guildContext.refetch();
                    if (canReadAudit) {
                        statistics.refetch();
                    }
                    if (canReadCases) {
                        cases.refetch();
                    }
                }}
            />
        );
    }

    return (
        <>
            <PageHeader
                description="Current moderation activity and work requiring attention."
                title="Overview"
            />
            {statistics.data ? (
                <section
                    aria-label="Moderation totals"
                    className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4"
                >
                    <MetricCard
                        label="Cases"
                        value={statistics.data.case_total}
                    />
                    <MetricCard
                        label="Actions"
                        value={statistics.data.action_total}
                    />
                    <MetricCard
                        label="Appeals"
                        value={statistics.data.appeal_total}
                    />
                    <MetricCard
                        label="Pending appeals"
                        value={
                            statistics.data.appeals_by_status.find(
                                (bucket) => bucket.key === "pending",
                            )?.count ?? 0
                        }
                    />
                </section>
            ) : null}
            {cases.data ? (
                <section className="flex flex-col gap-3">
                    <div>
                        <h2 className="font-heading font-semibold text-lg">
                            Recent cases
                        </h2>
                        <p className="text-muted-foreground text-sm">
                            The latest moderation decisions in this server.
                        </p>
                    </div>
                    <CaseTable cases={cases.data.cases} guildId={guildId} />
                </section>
            ) : null}
            {!canReadAudit && !canReadCases ? (
                <Alert>
                    <AlertTitle>Configuration access</AlertTitle>
                    <AlertDescription>
                        Your current Discord permissions allow configuration,
                        but not moderation history. Use Templates or Settings
                        from the sidebar.
                    </AlertDescription>
                </Alert>
            ) : null}
        </>
    );
}

function MetricCard({ label, value }: { label: string; value: number }) {
    return (
        <Card>
            <CardHeader>
                <p className="text-muted-foreground text-sm">{label}</p>
                <CardTitle className="text-3xl tabular-nums">{value}</CardTitle>
            </CardHeader>
        </Card>
    );
}
