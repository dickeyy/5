import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { PageHeader } from "#/components/dashboard/page-header";
import { QueryError, QueryPending } from "#/components/dashboard/query-state";
import { Badge } from "#/components/ui/badge";
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
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "#/components/ui/table";
import type { AuditLogFilters } from "#/lib/api";
import { formatDateTime, formatIdentifier, titleCase } from "#/lib/format";
import { auditLogQuery } from "#/lib/queries";

export const Route = createFileRoute("/guilds/$guildId/audit-log")({
    component: AuditLogPage,
});

function AuditLogPage() {
    const { guildId } = Route.useParams();
    const [filters, setFilters] = useState<AuditLogFilters>({});
    const [offset, setOffset] = useState(0);
    const pageSize = 25;
    const auditLog = useQuery(
        auditLogQuery(guildId, pageSize, offset, filters),
    );

    return (
        <>
            <PageHeader
                description="Permanent moderation and administration history."
                title="Audit log"
            />
            <form
                className="grid items-end gap-3 rounded-xl border p-4 sm:grid-cols-2 lg:grid-cols-[minmax(12rem,1fr)_minmax(12rem,1fr)_12rem_auto]"
                onSubmit={(event) => {
                    event.preventDefault();
                    const formData = new FormData(event.currentTarget);
                    const result = String(formData.get("result") ?? "");
                    setFilters({
                        action:
                            String(formData.get("action") ?? "").trim() ||
                            undefined,
                        resourceType:
                            String(
                                formData.get("resource_type") ?? "",
                            ).trim() || undefined,
                        result: result === "all" ? undefined : result,
                    });
                    setOffset(0);
                }}
            >
                <Field name="action">
                    <FieldLabel>Action</FieldLabel>
                    <Input name="action" placeholder="All actions" />
                </Field>
                <Field name="resource_type">
                    <FieldLabel>Resource type</FieldLabel>
                    <Input name="resource_type" placeholder="All resources" />
                </Field>
                <Field name="result">
                    <FieldLabel>Result</FieldLabel>
                    <Select
                        defaultValue="all"
                        items={[
                            { label: "All results", value: "all" },
                            { label: "Success", value: "success" },
                            { label: "Failure", value: "failure" },
                            { label: "Denied", value: "denied" },
                        ]}
                        name="result"
                    >
                        <SelectTrigger>
                            <SelectValue />
                        </SelectTrigger>
                        <SelectPopup>
                            <SelectItem value="all">All results</SelectItem>
                            <SelectItem value="success">Success</SelectItem>
                            <SelectItem value="failure">Failure</SelectItem>
                            <SelectItem value="denied">Denied</SelectItem>
                        </SelectPopup>
                    </Select>
                </Field>
                <Button type="submit" variant="outline">
                    Apply filters
                </Button>
            </form>
            {auditLog.isPending ? (
                <QueryPending label="Loading audit log" />
            ) : auditLog.isError ? (
                <QueryError onRetry={() => auditLog.refetch()} />
            ) : (
                <>
                    <Table variant="card">
                        <TableHeader>
                            <TableRow>
                                <TableHead>Action</TableHead>
                                <TableHead>Resource</TableHead>
                                <TableHead>Result</TableHead>
                                <TableHead className="max-lg:hidden">
                                    Actor
                                </TableHead>
                                <TableHead className="max-sm:hidden">
                                    Created
                                </TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {auditLog.data.entries.length === 0 ? (
                                <TableRow>
                                    <TableCell
                                        className="h-28 text-center text-muted-foreground"
                                        colSpan={5}
                                    >
                                        No audit entries found.
                                    </TableCell>
                                </TableRow>
                            ) : (
                                auditLog.data.entries.map((entry) => (
                                    <TableRow key={entry.id}>
                                        <TableCell className="font-medium">
                                            {titleCase(entry.action)}
                                        </TableCell>
                                        <TableCell>
                                            <span>
                                                {titleCase(entry.resource_type)}
                                            </span>
                                            <span className="ml-2 font-mono text-muted-foreground text-xs">
                                                {formatIdentifier(
                                                    entry.resource_id,
                                                )}
                                            </span>
                                        </TableCell>
                                        <TableCell>
                                            <Badge
                                                variant={
                                                    entry.result === "success"
                                                        ? "success"
                                                        : entry.result ===
                                                            "denied"
                                                          ? "warning"
                                                          : "error"
                                                }
                                            >
                                                {titleCase(entry.result)}
                                            </Badge>
                                        </TableCell>
                                        <TableCell className="font-mono text-xs max-lg:hidden">
                                            {entry.actor_discord_user_id
                                                ? formatIdentifier(
                                                      entry.actor_discord_user_id,
                                                  )
                                                : titleCase(entry.source)}
                                        </TableCell>
                                        <TableCell className="text-muted-foreground max-sm:hidden">
                                            {formatDateTime(entry.created_at)}
                                        </TableCell>
                                    </TableRow>
                                ))
                            )}
                        </TableBody>
                    </Table>
                    <div className="flex items-center justify-between gap-3">
                        <p className="text-muted-foreground text-sm">
                            {auditLog.data.total === 0
                                ? "No results"
                                : `${offset + 1}–${Math.min(
                                      offset + auditLog.data.entries.length,
                                      auditLog.data.total,
                                  )} of ${auditLog.data.total}`}
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
                                variant="outline"
                            >
                                Previous
                            </Button>
                            <Button
                                disabled={
                                    offset + auditLog.data.entries.length >=
                                    auditLog.data.total
                                }
                                onClick={() =>
                                    setOffset((current) => current + pageSize)
                                }
                                size="sm"
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
