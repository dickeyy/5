import { useMutation, useQueryClient } from "@tanstack/react-query";
import { PlusIcon } from "lucide-react";
import { useState } from "react";
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
import {
    Select,
    SelectItem,
    SelectPopup,
    SelectTrigger,
    SelectValue,
} from "#/components/ui/select";
import { Textarea } from "#/components/ui/textarea";
import { createCase } from "#/lib/api";
import type {
    CaseCreateInput,
    Template,
    TemplateContextField,
} from "#/lib/api-types";

type CreateCaseDialogProps = {
    csrfToken: string;
    guildId: string;
    templates: Template[];
};

export function CreateCaseDialog({
    csrfToken,
    guildId,
    templates,
}: CreateCaseDialogProps) {
    const queryClient = useQueryClient();
    const [open, setOpen] = useState(false);
    const [templateId, setTemplateId] = useState<string | null>(null);
    const [contextValues, setContextValues] = useState<
        Record<string, string | boolean>
    >({});
    const activeTemplates = templates.filter(
        (template) => !template.archived_at,
    );
    const selectedTemplate = activeTemplates.find(
        (template) => template.id === templateId,
    );
    const templateItems = activeTemplates.map((template) => ({
        label: template.name,
        value: template.id,
    }));
    const mutation = useMutation({
        mutationFn: (input: CaseCreateInput) =>
            createCase(guildId, input, csrfToken),
        onSuccess: async () => {
            await Promise.all([
                queryClient.invalidateQueries({
                    queryKey: ["guild", guildId, "cases"],
                }),
                queryClient.invalidateQueries({
                    queryKey: ["guild", guildId, "statistics"],
                }),
            ]);
            setOpen(false);
            setTemplateId(null);
            setContextValues({});
        },
    });

    function selectTemplate(value: string | null) {
        setTemplateId(value);
        const template = activeTemplates.find((item) => item.id === value);
        setContextValues(
            Object.fromEntries(
                (template?.context_fields ?? []).map((field) => [
                    field.key,
                    field.type === "boolean" ? false : "",
                ]),
            ),
        );
    }

    function submit(event: React.FormEvent<HTMLFormElement>) {
        event.preventDefault();
        if (!selectedTemplate) {
            return;
        }

        const formData = new FormData(event.currentTarget);
        const targetDiscordUserId = String(
            formData.get("target_discord_user_id") ?? "",
        ).trim();
        mutation.mutate({
            context_values: selectedTemplate.context_fields.map((field) => ({
                key: field.key,
                value: contextValue(field, contextValues[field.key]),
            })),
            target_discord_user_id: targetDiscordUserId,
            template_id: selectedTemplate.id,
        });
    }

    return (
        <Dialog onOpenChange={setOpen} open={open}>
            <DialogTrigger render={<Button />}>
                <PlusIcon aria-hidden="true" />
                Create case
            </DialogTrigger>
            <DialogPopup>
                <DialogHeader>
                    <DialogTitle>Create case</DialogTitle>
                    <DialogDescription>
                        Apply an active template to a Discord member. Quack
                        selects the escalation level.
                    </DialogDescription>
                </DialogHeader>
                <form className="contents" onSubmit={submit}>
                    <DialogPanel className="flex flex-col gap-5">
                        <Field name="template_id">
                            <FieldLabel>Template</FieldLabel>
                            <Select
                                items={templateItems}
                                onValueChange={selectTemplate}
                                required
                                value={templateId}
                            >
                                <SelectTrigger>
                                    <SelectValue placeholder="Choose a template" />
                                </SelectTrigger>
                                <SelectPopup>
                                    {templateItems.map((item) => (
                                        <SelectItem
                                            key={item.value}
                                            value={item.value}
                                        >
                                            {item.label}
                                        </SelectItem>
                                    ))}
                                </SelectPopup>
                            </Select>
                            <FieldDescription>
                                Archived templates cannot create new cases.
                            </FieldDescription>
                        </Field>
                        <Field name="target_discord_user_id">
                            <FieldLabel>Discord user ID</FieldLabel>
                            <Input
                                autoComplete="off"
                                name="target_discord_user_id"
                                placeholder="123456789012345678"
                                required
                                type="text"
                            />
                        </Field>
                        {selectedTemplate?.context_fields
                            .slice()
                            .sort(
                                (left, right) => left.position - right.position,
                            )
                            .map((field) => (
                                <ContextField
                                    field={field}
                                    key={field.id}
                                    onChange={(value) =>
                                        setContextValues((current) => ({
                                            ...current,
                                            [field.key]: value,
                                        }))
                                    }
                                    value={contextValues[field.key]}
                                />
                            ))}
                        {mutation.isError ? (
                            <Alert variant="error">
                                <AlertTitle>Case was not created</AlertTitle>
                                <AlertDescription>
                                    {mutation.error.message}
                                </AlertDescription>
                            </Alert>
                        ) : null}
                    </DialogPanel>
                    <DialogFooter>
                        <DialogClose render={<Button variant="ghost" />}>
                            Cancel
                        </DialogClose>
                        <Button
                            disabled={!selectedTemplate}
                            loading={mutation.isPending}
                            type="submit"
                        >
                            Create case
                        </Button>
                    </DialogFooter>
                </form>
            </DialogPopup>
        </Dialog>
    );
}

function ContextField({
    field,
    onChange,
    value,
}: {
    field: TemplateContextField;
    onChange: (value: string | boolean) => void;
    value: string | boolean | undefined;
}) {
    if (field.type === "boolean") {
        const checked = value === true;
        return (
            <Field name={`context_${field.key}`}>
                <div className="flex w-full items-center justify-between gap-4 rounded-lg border px-3 py-2.5">
                    <div className="flex flex-col gap-1">
                        <FieldLabel>{field.label}</FieldLabel>
                        <FieldDescription>
                            {field.required
                                ? "A value is required."
                                : "Optional context visible to the member."}
                        </FieldDescription>
                    </div>
                    <Checkbox
                        aria-label={field.label}
                        checked={checked}
                        onCheckedChange={(nextValue) =>
                            onChange(nextValue === true)
                        }
                    />
                </div>
            </Field>
        );
    }

    const commonProps = {
        name: `context_${field.key}`,
        onChange: (
            event: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>,
        ) => onChange(event.currentTarget.value),
        required: field.required,
        value: typeof value === "string" ? value : "",
    };

    return (
        <Field name={`context_${field.key}`}>
            <FieldLabel>{field.label}</FieldLabel>
            {field.type === "long_text" ? (
                <Textarea {...commonProps} />
            ) : (
                <Input
                    {...commonProps}
                    inputMode={field.type === "number" ? "decimal" : undefined}
                    placeholder={
                        field.type === "discord_message_link"
                            ? "https://discord.com/channels/…"
                            : undefined
                    }
                    type={
                        field.type === "number"
                            ? "number"
                            : field.type === "discord_message_link"
                              ? "url"
                              : "text"
                    }
                />
            )}
            <FieldDescription>
                {field.required
                    ? "Required and visible to the member."
                    : "Optional and visible to the member."}
            </FieldDescription>
        </Field>
    );
}

function contextValue(
    field: TemplateContextField,
    value: string | boolean | undefined,
): unknown {
    if (field.type === "boolean") {
        return value === true;
    }
    if (field.type === "number") {
        return value === "" || value === undefined ? null : Number(value);
    }
    return typeof value === "string" ? value.trim() : "";
}
