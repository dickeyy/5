import { useMutation, useQueryClient } from "@tanstack/react-query";
import { DownloadIcon, UploadIcon } from "lucide-react";
import { useId, useState } from "react";
import { Alert, AlertDescription, AlertTitle } from "#/components/ui/alert";
import { Button } from "#/components/ui/button";
import { Checkbox } from "#/components/ui/checkbox";
import {
    Dialog,
    DialogClose,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogPanel,
    DialogPopup,
    DialogTitle,
    DialogTrigger,
} from "#/components/ui/dialog";
import { Field, FieldDescription, FieldLabel } from "#/components/ui/field";
import { Input } from "#/components/ui/input";
import { exportTemplate, importTemplate } from "#/lib/api";
import type { Template, TemplatePolicy } from "#/lib/api-types";

export function ExportTemplateButton({
    guildId,
    template,
}: {
    guildId: string;
    template: Template;
}) {
    const mutation = useMutation({
        mutationFn: () => exportTemplate(guildId, template.id),
        onSuccess: (policy) => {
            const contents = JSON.stringify(policy, null, 2);
            const url = URL.createObjectURL(
                new Blob([contents], { type: "application/json" }),
            );
            const link = document.createElement("a");
            link.href = url;
            link.download = `${template.slug}.quack-policy.json`;
            link.click();
            URL.revokeObjectURL(url);
        },
    });

    return (
        <div className="flex flex-col items-end gap-1">
            <Button
                loading={mutation.isPending}
                onClick={() => mutation.mutate()}
                variant="outline"
            >
                <DownloadIcon aria-hidden="true" />
                Export
            </Button>
            {mutation.isError ? (
                <span className="text-destructive text-xs" role="alert">
                    {mutation.error.message}
                </span>
            ) : null}
        </div>
    );
}

export function ImportTemplateDialog({
    csrfToken,
    guildId,
}: {
    csrfToken: string;
    guildId: string;
}) {
    const queryClient = useQueryClient();
    const confirmationId = useId();
    const [open, setOpen] = useState(false);
    const [confirmed, setConfirmed] = useState(false);
    const [policy, setPolicy] = useState<TemplatePolicy | null>(null);
    const [fileError, setFileError] = useState<string | null>(null);
    const mutation = useMutation({
        mutationFn: (input: TemplatePolicy) =>
            importTemplate(guildId, input, csrfToken),
        onSuccess: async () => {
            await queryClient.invalidateQueries({
                queryKey: ["guild", guildId, "templates"],
            });
            setOpen(false);
            setConfirmed(false);
            setPolicy(null);
            setFileError(null);
        },
    });

    return (
        <Dialog onOpenChange={setOpen} open={open}>
            <DialogTrigger render={<Button variant="outline" />}>
                <UploadIcon aria-hidden="true" />
                Import
            </DialogTrigger>
            <DialogPopup>
                <DialogHeader>
                    <DialogTitle>Import template</DialogTitle>
                    <DialogDescription>
                        Import a Quack policy export as a new active template in
                        this server.
                    </DialogDescription>
                </DialogHeader>
                <form
                    className="contents"
                    onSubmit={(event) => {
                        event.preventDefault();
                        if (policy && confirmed) {
                            mutation.mutate(policy);
                        }
                    }}
                >
                    <DialogPanel className="flex flex-col gap-5">
                        <Field name="policy-file">
                            <FieldLabel>Policy file</FieldLabel>
                            <Input
                                accept="application/json,.json"
                                nativeInput
                                onChange={async (event) => {
                                    setPolicy(null);
                                    setFileError(null);
                                    const file = event.target.files?.[0];
                                    if (!file) {
                                        return;
                                    }
                                    try {
                                        const parsed = JSON.parse(
                                            await file.text(),
                                        ) as unknown;
                                        if (!isTemplatePolicy(parsed)) {
                                            throw new Error(
                                                "This file is not a Quack template policy.",
                                            );
                                        }
                                        setPolicy(parsed);
                                    } catch (error) {
                                        setFileError(
                                            error instanceof Error
                                                ? error.message
                                                : "The policy file could not be read.",
                                        );
                                    }
                                }}
                                required
                                type="file"
                            />
                            <FieldDescription>
                                JSON exported from a Quack template.
                            </FieldDescription>
                        </Field>
                        {policy ? (
                            <Alert variant="info">
                                <AlertTitle>{policy.name}</AlertTitle>
                                <AlertDescription>
                                    Policy version {policy.schema_version} ·{" "}
                                    {policy.levels.length} escalation levels
                                </AlertDescription>
                            </Alert>
                        ) : null}
                        {fileError || mutation.isError ? (
                            <Alert variant="error">
                                <AlertTitle>
                                    Template could not be imported
                                </AlertTitle>
                                <AlertDescription>
                                    {fileError ?? mutation.error?.message}
                                </AlertDescription>
                            </Alert>
                        ) : null}
                        <label
                            className="flex items-start gap-3 text-sm"
                            htmlFor={confirmationId}
                        >
                            <Checkbox
                                checked={confirmed}
                                id={confirmationId}
                                onCheckedChange={setConfirmed}
                            />
                            <span>
                                Create this policy as a new active template in
                                the current server.
                            </span>
                        </label>
                    </DialogPanel>
                    <DialogFooter>
                        <DialogClose render={<Button variant="ghost" />}>
                            Cancel
                        </DialogClose>
                        <Button
                            disabled={!policy || !confirmed}
                            loading={mutation.isPending}
                            type="submit"
                        >
                            Import template
                        </Button>
                    </DialogFooter>
                </form>
            </DialogPopup>
        </Dialog>
    );
}

function isTemplatePolicy(value: unknown): value is TemplatePolicy {
    if (!value || typeof value !== "object") {
        return false;
    }
    const policy = value as Partial<TemplatePolicy>;
    return (
        policy.schema_version === 1 &&
        typeof policy.slug === "string" &&
        typeof policy.name === "string" &&
        typeof policy.official_reason === "string" &&
        typeof policy.appealable === "boolean" &&
        Array.isArray(policy.context_fields) &&
        Array.isArray(policy.levels)
    );
}
