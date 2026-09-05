import { Link } from "@tanstack/react-router";
import { Badge } from "#/components/ui/badge";
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "#/components/ui/table";
import type { Case } from "#/lib/api-types";
import { formatDateTime, formatIdentifier, titleCase } from "#/lib/format";

export function CaseTable({
    cases,
    guildId,
}: {
    cases: Case[];
    guildId?: string;
}) {
    return (
        <Table variant="card">
            <TableHeader>
                <TableRow>
                    <TableHead>Case</TableHead>
                    <TableHead>Member</TableHead>
                    <TableHead>Reason</TableHead>
                    <TableHead className="max-md:hidden">Outcome</TableHead>
                    <TableHead className="max-sm:hidden">Created</TableHead>
                </TableRow>
            </TableHeader>
            <TableBody>
                {cases.length === 0 ? (
                    <TableRow>
                        <TableCell
                            className="h-28 text-center text-muted-foreground"
                            colSpan={5}
                        >
                            No cases found.
                        </TableCell>
                    </TableRow>
                ) : (
                    cases.map((caseItem) => (
                        <TableRow key={caseItem.id}>
                            <TableCell>
                                <div className="flex items-center gap-2">
                                    {guildId ? (
                                        <Link
                                            className="font-medium tabular-nums underline-offset-4 hover:underline"
                                            params={{
                                                caseRef: String(
                                                    caseItem.case_number,
                                                ),
                                                guildId,
                                            }}
                                            to="/guilds/$guildId/cases/$caseRef"
                                        >
                                            #{caseItem.case_number}
                                        </Link>
                                    ) : (
                                        <span className="font-medium tabular-nums">
                                            #{caseItem.case_number}
                                        </span>
                                    )}
                                    {caseItem.validity === "voided" ? (
                                        <Badge variant="secondary">
                                            Voided
                                        </Badge>
                                    ) : null}
                                </div>
                            </TableCell>
                            <TableCell className="font-mono text-xs">
                                {guildId ? (
                                    <Link
                                        className="underline-offset-4 hover:underline"
                                        params={{
                                            guildId,
                                            memberId:
                                                caseItem.target_discord_user_id,
                                        }}
                                        to="/guilds/$guildId/members/$memberId"
                                    >
                                        {formatIdentifier(
                                            caseItem.target_discord_user_id,
                                        )}
                                    </Link>
                                ) : (
                                    formatIdentifier(
                                        caseItem.target_discord_user_id,
                                    )
                                )}
                            </TableCell>
                            <TableCell className="max-w-72 truncate">
                                {caseItem.reason}
                            </TableCell>
                            <TableCell className="max-md:hidden">
                                {caseItem.selected_level?.name ??
                                    titleCase(
                                        caseItem.actions[0]?.action_type ??
                                            "Case only",
                                    )}
                            </TableCell>
                            <TableCell className="text-muted-foreground max-sm:hidden">
                                {formatDateTime(caseItem.created_at)}
                            </TableCell>
                        </TableRow>
                    ))
                )}
            </TableBody>
        </Table>
    );
}
