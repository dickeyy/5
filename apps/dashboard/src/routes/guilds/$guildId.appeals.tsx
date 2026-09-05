import { useQuery } from "@tanstack/react-query";
import { createFileRoute, Link } from "@tanstack/react-router";
import { useState } from "react";
import { PageHeader } from "#/components/dashboard/page-header";
import { QueryError, QueryPending } from "#/components/dashboard/query-state";
import { Badge } from "#/components/ui/badge";
import { Button } from "#/components/ui/button";
import { Field, FieldLabel } from "#/components/ui/field";
import {
    Select,
    SelectItem,
    SelectPopup,
    SelectTrigger,
    SelectValue,
} from "#/components/ui/select";
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "#/components/ui/table";
import { AppealReviewDialog } from "#/features/appeals/appeal-review-dialog";
import { formatDateTime, formatIdentifier, titleCase } from "#/lib/format";
import { appealsQuery, guildContextQuery, sessionQuery } from "#/lib/queries";

export const Route = createFileRoute("/guilds/$guildId/appeals")({
    component: AppealsPage,
});

function AppealsPage() {
    const { guildId } = Route.useParams();
    const [status, setStatus] = useState("all");
    const [draftStatus, setDraftStatus] = useState("all");
    const [offset, setOffset] = useState(0);
    const pageSize = 25;
    const appeals = useQuery(
        appealsQuery(
            guildId,
            pageSize,
            offset,
            status === "all" ? undefined : status,
        ),
    );
    const guildContext = useQuery(guildContextQuery(guildId));
    const session = useQuery(sessionQuery);
    const canReview = Boolean(guildContext.data?.permissions["appeal.review"]);

    return (
        <>
            <PageHeader
                description="Case appeals awaiting or recording staff review."
                title="Appeals"
            />
            <form
                className="flex items-end gap-3 rounded-xl border p-4"
                onSubmit={(event) => {
                    event.preventDefault();
                    setStatus(draftStatus);
                    setOffset(0);
                }}
            >
                <Field className="w-full max-w-xs" name="status">
                    <FieldLabel>Status</FieldLabel>
                    <Select
                        items={appealStatusItems}
                        onValueChange={(value) =>
                            value ? setDraftStatus(value) : undefined
                        }
                        value={draftStatus}
                    >
                        <SelectTrigger>
                            <SelectValue />
                        </SelectTrigger>
                        <SelectPopup>
                            {appealStatusItems.map((item) => (
                                <SelectItem key={item.value} value={item.value}>
                                    {item.label}
                                </SelectItem>
                            ))}
                        </SelectPopup>
                    </Select>
                </Field>
                <Button type="submit" variant="outline">
                    Apply filter
                </Button>
            </form>
            {appeals.isPending ? (
                <QueryPending label="Loading appeals" />
            ) : appeals.isError ? (
                <QueryError onRetry={() => appeals.refetch()} />
            ) : (
                <>
                    <Table variant="card">
                        <TableHeader>
                            <TableRow>
                                <TableHead>Case</TableHead>
                                <TableHead>Member</TableHead>
                                <TableHead>Status</TableHead>
                                <TableHead className="max-sm:hidden">
                                    Updated
                                </TableHead>
                                <TableHead>
                                    <span className="sr-only">Actions</span>
                                </TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {appeals.data.appeals.length === 0 ? (
                                <TableRow>
                                    <TableCell
                                        className="h-28 text-center text-muted-foreground"
                                        colSpan={5}
                                    >
                                        No appeals found.
                                    </TableCell>
                                </TableRow>
                            ) : (
                                appeals.data.appeals.map((appeal) => (
                                    <TableRow key={appeal.id}>
                                        <TableCell className="font-mono text-xs">
                                            <Link
                                                className="underline-offset-4 hover:underline"
                                                params={{
                                                    caseRef: appeal.case_id,
                                                    guildId,
                                                }}
                                                to="/guilds/$guildId/cases/$caseRef"
                                            >
                                                {formatIdentifier(
                                                    appeal.case_id,
                                                )}
                                            </Link>
                                        </TableCell>
                                        <TableCell className="font-mono text-xs">
                                            <Link
                                                className="underline-offset-4 hover:underline"
                                                params={{
                                                    guildId,
                                                    memberId:
                                                        appeal.target_discord_user_id,
                                                }}
                                                to="/guilds/$guildId/members/$memberId"
                                            >
                                                {formatIdentifier(
                                                    appeal.target_discord_user_id,
                                                )}
                                            </Link>
                                        </TableCell>
                                        <TableCell>
                                            <Badge
                                                variant={appealStatusVariant(
                                                    appeal.status,
                                                )}
                                            >
                                                {titleCase(appeal.status)}
                                            </Badge>
                                        </TableCell>
                                        <TableCell className="text-muted-foreground max-sm:hidden">
                                            {formatDateTime(appeal.updated_at)}
                                        </TableCell>
                                        <TableCell className="text-right">
                                            {canReview && session.data ? (
                                                <AppealReviewDialog
                                                    key={`${appeal.id}:${appeal.status}`}
                                                    appeal={appeal}
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
                            {appeals.data.total === 0
                                ? "No results"
                                : `${offset + 1}–${Math.min(
                                      offset + appeals.data.appeals.length,
                                      appeals.data.total,
                                  )} of ${appeals.data.total}`}
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
                                    offset + appeals.data.appeals.length >=
                                    appeals.data.total
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

const appealStatusItems = [
    { label: "All appeals", value: "all" },
    { label: "Pending", value: "pending" },
    { label: "Needs information", value: "needs_information" },
    { label: "Accepted", value: "accepted" },
    { label: "Rejected", value: "rejected" },
    { label: "Closed", value: "closed" },
];

function appealStatusVariant(status: string) {
    switch (status) {
        case "accepted":
            return "success" as const;
        case "rejected":
        case "closed":
            return "secondary" as const;
        case "needs_information":
            return "warning" as const;
        default:
            return "info" as const;
    }
}
