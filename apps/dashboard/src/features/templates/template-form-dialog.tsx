import { useMutation, useQueryClient } from "@tanstack/react-query";
import { PencilIcon, PlusIcon } from "lucide-react";
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
import { Textarea } from "#/components/ui/textarea";
import { createTemplate, updateTemplate } from "#/lib/api";
import type { Template } from "#/lib/api-types";
import { ContextFieldsEditor } from "./context-fields-editor";
import { LevelsEditor } from "./levels-editor";
import {
    createTemplateDraft,
    type TemplateDraft,
    templateInputFromDraft,
} from "./template-form-state";

type TemplateFormDialogProps = {
    csrfToken: string;
    guildId: string;
    template?: Template;
};

export function TemplateFormDialog({
    csrfToken,
    guildId,
    template,
}: TemplateFormDialogProps) {
    const queryClient = useQueryClient();
    const appealableId = useId();
    const [open, setOpen] = useState(false);
    const [draft, setDraft] = useState(() => createTemplateDraft(template));
    const updateDraft = (patch: Partial<TemplateDraft>) => {
        setDraft((current) => ({ ...current, ...patch }));
    };
    const mutation = useMutation({
        mutationFn: () => {
            const input = templateInputFromDraft(draft);
            return template
                ? updateTemplate(guildId, template.id, input, csrfToken)
                : createTemplate(guildId, input, csrfToken);
        },
        onSuccess: async () => {
            await queryClient.invalidateQueries({
                queryKey: ["guild", guildId, "templates"],
            });
            setOpen(false);
            if (!template) {
                setDraft(createTemplateDraft());
            }
        },
    });

    return (
        <Dialog onOpenChange={setOpen} open={open}>
            <DialogTrigger
                render={<Button variant={template ? "outline" : "default"} />}
            >
                {template ? (
                    <PencilIcon aria-hidden="true" />
                ) : (
                    <PlusIcon aria-hidden="true" />
                )}
                {template ? "Edit" : "New template"}
            </DialogTrigger>
            <DialogPopup className="max-w-3xl">
                <DialogHeader>
                    <DialogTitle>
                        {template ? `Edit ${template.name}` : "New template"}
                    </DialogTitle>
                    <DialogDescription>
                        Define the rule, required context, and automatic
                        escalation applied to future cases.
                    </DialogDescription>
                </DialogHeader>
                <form
                    className="contents"
                    onSubmit={(event) => {
                        event.preventDefault();
                        mutation.mutate();
                    }}
                >
                    <DialogPanel className="flex flex-col gap-6">
                        <div className="grid gap-4 sm:grid-cols-2">
                            <Field name="name">
                                <FieldLabel>Name</FieldLabel>
                                <Input
                                    onChange={(event) =>
                                        updateDraft({
                                            name: event.currentTarget.value,
                                        })
                                    }
                                    placeholder="Spam"
                                    required
                                    type="text"
                                    value={draft.name}
                                />
                            </Field>
                            <Field name="slug">
                                <FieldLabel>Slug</FieldLabel>
                                <Input
                                    maxLength={64}
                                    onChange={(event) =>
                                        updateDraft({
                                            slug: event.currentTarget.value,
                                        })
                                    }
                                    pattern="[a-z0-9_-]{2,64}"
                                    placeholder="spam"
                                    required
                                    type="text"
                                    value={draft.slug}
                                />
                            </Field>
                        </div>
                        <Field name="description">
                            <FieldLabel>Description</FieldLabel>
                            <Textarea
                                onChange={(event) =>
                                    updateDraft({
                                        description: event.currentTarget.value,
                                    })
                                }
                                placeholder="When moderators should apply this rule."
                                value={draft.description}
                            />
                        </Field>
                        <Field name="reason_template">
                            <FieldLabel>Official reason</FieldLabel>
                            <Textarea
                                onChange={(event) =>
                                    updateDraft({
                                        reasonTemplate:
                                            event.currentTarget.value,
                                    })
                                }
                                placeholder="The reason shown to the affected member."
                                required
                                value={draft.reasonTemplate}
                            />
                            <FieldDescription>
                                Moderators cannot replace this reason while
                                creating a case.
                            </FieldDescription>
                        </Field>
                        <label
                            className="flex items-center gap-2 text-sm"
                            htmlFor={appealableId}
                        >
                            <Checkbox
                                checked={draft.appealable}
                                id={appealableId}
                                onCheckedChange={(checked) =>
                                    updateDraft({
                                        appealable: checked === true,
                                    })
                                }
                            />
                            Cases from this template can be appealed
                        </label>
                        <ContextFieldsEditor
                            fields={draft.contextFields}
                            onChange={(contextFields) =>
                                updateDraft({
                                    contextFields,
                                })
                            }
                        />
                        <LevelsEditor
                            levels={draft.levels}
                            onChange={(levels) =>
                                updateDraft({
                                    levels,
                                })
                            }
                        />
                        {mutation.isError ? (
                            <Alert variant="error">
                                <AlertTitle>Template was not saved</AlertTitle>
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
                        <Button loading={mutation.isPending} type="submit">
                            Save template
                        </Button>
                    </DialogFooter>
                </form>
            </DialogPopup>
        </Dialog>
    );
}
