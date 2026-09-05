import { useQuery } from "@tanstack/react-query";
import { createFileRoute, Link } from "@tanstack/react-router";
import { useState } from "react";
import { PageHeader } from "#/components/dashboard/page-header";
import { QueryError, QueryPending } from "#/components/dashboard/query-state";
import { Badge } from "#/components/ui/badge";
import { Button } from "#/components/ui/button";
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "#/components/ui/table";
import { FailureActions } from "#/features/actions/failure-actions";
import { formatDateTime, formatIdentifier, titleCase } from "#/lib/format";
import {
    actionFailuresQuery,
    guildContextQuery,
    sessionQuery,
} from "#/lib/queries";

export const Route = createFileRoute("/guilds/$guildId/action-failures")({
    component: ActionFailuresPage,
});

function ActionFailuresPage() {
    const { guildId } = Route.useParams();
    const [offset, setOffset] = useState(0);
    const pageSize = 25;
    const failures = useQuery(actionFailuresQuery(guildId, pageSize, offset));
    const guildContext = useQuery(guildContextQuery(guildId));
    const session = useQuery(sessionQuery);
    const canRetry = Boolean(guildContext.data?.permissions["case.create"]);
    const canDismiss = Boolean(
        guildContext.data?.permissions["action_failure.dismiss"],
    );

    return (
        <>
            <PageHeader
                description="Discord actions that need staff review or recovery."
                title="Action failures"
            />
            {failures.isPending ? (
                <QueryPending label="Loading action failures" />
            ) : failures.isError ? (
                <QueryError onRetry={() => failures.refetch()} />
            ) : (
                <>
                    <Table variant="card">
                        <TableHeader>
                            <TableRow>
                                <TableHead>Case</TableHead>
                                <TableHead>Action</TableHead>
                                <TableHead className="max-md:hidden">
                                    Failure
                                </TableHead>
                                <TableHead className="max-sm:hidden">
                                    Updated
                                </TableHead>
                                <TableHead>
                                    <span className="sr-only">Actions</span>
                                </TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {failures.data.executions.length === 0 ? (
                                <TableRow>
                                    <TableCell
                                        className="h-28 text-center text-muted-foreground"
                                        colSpan={5}
                                    >
                                        No action failures need review.
                                    </TableCell>
                                </TableRow>
                            ) : (
                                failures.data.executions.map((action) => (
                                    <TableRow key={action.id}>
                                        <TableCell className="font-mono text-xs">
                                            <Link
                                                className="underline-offset-4 hover:underline"
                                                params={{
                                                    caseRef: action.case_id,
                                                    guildId,
                                                }}
                                                to="/guilds/$guildId/cases/$caseRef"
                                            >
                                                {formatIdentifier(
                                                    action.case_id,
                                                )}
                                            </Link>
                                        </TableCell>
                                        <TableCell>
                                            <div className="flex items-center gap-2">
                                                <span>
                                                    {titleCase(
                                                        action.action_type,
                                                    )}
                                                </span>
                                                <Badge variant="error">
                                                    {titleCase(action.status)}
                                                </Badge>
                                            </div>
                                        </TableCell>
                                        <TableCell className="max-w-80 truncate text-muted-foreground max-md:hidden">
                                            {action.last_error ||
                                                action.last_error_code ||
                                                "Action failed"}
                                        </TableCell>
                                        <TableCell className="text-muted-foreground max-sm:hidden">
                                            {formatDateTime(action.updated_at)}
                                        </TableCell>
                                        <TableCell>
                                            {session.data ? (
                                                <FailureActions
                                                    action={action}
                                                    canDismiss={canDismiss}
                                                    canRetry={canRetry}
                                                    csrfToken={
                                                        session.data.csrf_token
                                                    }
                                                    guildId={guildId}
                                                />
                                            ) : null}
                                        </TableCell>
                                    </TableRow>
                                ))
                            )}
                        </TableBody>
                    </Table>
                    <div className="flex items-center justify-between gap-3">
                        <p className="text-muted-foreground text-sm">
                            {failures.data.total === 0
                                ? "No results"
                                : `${offset + 1}–${Math.min(
                                      offset + failures.data.executions.length,
                                      failures.data.total,
                                  )} of ${failures.data.total}`}
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
                                    offset + failures.data.executions.length >=
                                    failures.data.total
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
