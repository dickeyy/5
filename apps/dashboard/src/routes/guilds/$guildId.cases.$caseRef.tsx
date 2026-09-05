import { useQuery } from "@tanstack/react-query";
import { createFileRoute, Link } from "@tanstack/react-router";
import { ArrowLeftIcon } from "lucide-react";
import { PageHeader } from "#/components/dashboard/page-header";
import { QueryError, QueryPending } from "#/components/dashboard/query-state";
import { Badge } from "#/components/ui/badge";
import { Button } from "#/components/ui/button";
import { CaseDetail } from "#/features/cases/case-detail";
import { VoidCaseDialog } from "#/features/cases/void-case-dialog";
import { caseQuery, guildContextQuery, sessionQuery } from "#/lib/queries";

export const Route = createFileRoute("/guilds/$guildId/cases/$caseRef")({
    component: CaseDetailPage,
});

function CaseDetailPage() {
    const { caseRef, guildId } = Route.useParams();
    const caseResult = useQuery(caseQuery(guildId, caseRef));
    const guildContext = useQuery(guildContextQuery(guildId));
    const session = useQuery(sessionQuery);

    if (caseResult.isPending) {
        return <QueryPending label="Loading case" />;
    }

    if (caseResult.isError) {
        return <QueryError onRetry={() => caseResult.refetch()} />;
    }

    const caseItem = caseResult.data;
    const canVoid =
        caseItem.validity !== "voided" &&
        Boolean(guildContext.data?.permissions["case.void"]) &&
        Boolean(session.data);
    const canReverse = Boolean(guildContext.data?.permissions["case.create"]);

    return (
        <>
            <PageHeader
                actions={
                    <div className="flex flex-wrap items-center gap-2">
                        <Badge
                            variant={
                                caseItem.validity === "voided"
                                    ? "secondary"
                                    : "success"
                            }
                        >
                            {caseItem.validity === "voided"
                                ? "Voided"
                                : "Valid"}
                        </Badge>
                        {canVoid && session.data ? (
                            <VoidCaseDialog
                                caseRef={caseRef}
                                csrfToken={session.data.csrf_token}
                                guildId={guildId}
                            />
                        ) : null}
                    </div>
                }
                description={`Created ${new Intl.DateTimeFormat(undefined, {
                    dateStyle: "medium",
                }).format(new Date(caseItem.created_at))}`}
                title={`Case #${caseItem.case_number}`}
            />
            <Button
                className="w-fit"
                render={
                    <Link params={{ guildId }} to="/guilds/$guildId/cases" />
                }
                size="sm"
                variant="ghost"
            >
                <ArrowLeftIcon aria-hidden="true" />
                All cases
            </Button>
            <CaseDetail
                caseItem={caseItem}
                reversal={
                    canReverse && session.data
                        ? {
                              caseRef,
                              csrfToken: session.data.csrf_token,
                              guildId,
                          }
                        : undefined
                }
            />
        </>
    );
}
