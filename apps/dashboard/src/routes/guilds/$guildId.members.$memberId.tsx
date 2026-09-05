import { useQuery } from "@tanstack/react-query";
import { createFileRoute, Link } from "@tanstack/react-router";
import { useState } from "react";
import { PageHeader } from "#/components/dashboard/page-header";
import { QueryError, QueryPending } from "#/components/dashboard/query-state";
import { Button } from "#/components/ui/button";
import { Card, CardHeader, CardTitle } from "#/components/ui/card";
import { CaseTable } from "#/features/cases/case-table";
import { formatIdentifier } from "#/lib/format";
import { memberCaseHistoryQuery } from "#/lib/queries";

export const Route = createFileRoute("/guilds/$guildId/members/$memberId")({
    component: MemberHistoryPage,
});

function MemberHistoryPage() {
    const { guildId, memberId } = Route.useParams();
    const [page, setPage] = useState(0);
    const pageSize = 25;
    const offset = page * pageSize;
    const history = useQuery(
        memberCaseHistoryQuery(guildId, memberId, pageSize, offset),
    );

    if (history.isPending) {
        return <QueryPending label="Loading member history" />;
    }

    if (history.isError) {
        return <QueryError onRetry={() => history.refetch()} />;
    }

    return (
        <>
            <PageHeader
                actions={
                    <Button
                        render={
                            <Link
                                params={{ guildId }}
                                to="/guilds/$guildId/cases"
                            />
                        }
                        size="sm"
                        variant="outline"
                    >
                        Back to cases
                    </Button>
                }
                description={`Moderation history for member ${formatIdentifier(memberId)}.`}
                title="Member history"
            />
            <section
                aria-label="Member case totals"
                className="grid gap-3 sm:grid-cols-3"
            >
                <MetricCard label="Total cases" value={history.data.total} />
                <MetricCard
                    label="Valid"
                    value={history.data.summary.by_validity.valid ?? 0}
                />
                <MetricCard
                    label="Voided"
                    value={history.data.summary.by_validity.voided ?? 0}
                />
            </section>
            <CaseTable cases={history.data.cases} guildId={guildId} />
            <div className="flex items-center justify-between gap-3">
                <p className="text-muted-foreground text-sm">
                    {history.data.total === 0
                        ? "No cases"
                        : `${offset + 1}–${Math.min(
                              offset + history.data.cases.length,
                              history.data.total,
                          )} of ${history.data.total}`}
                </p>
                <div className="flex gap-2">
                    <Button
                        disabled={page === 0}
                        onClick={() =>
                            setPage((current) => Math.max(0, current - 1))
                        }
                        size="sm"
                        type="button"
                        variant="outline"
                    >
                        Previous
                    </Button>
                    <Button
                        disabled={
                            offset + history.data.cases.length >=
                            history.data.total
                        }
                        onClick={() => setPage((current) => current + 1)}
                        size="sm"
                        type="button"
                        variant="outline"
                    >
                        Next
                    </Button>
                </div>
            </div>
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
