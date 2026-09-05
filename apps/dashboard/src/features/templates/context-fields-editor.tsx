import { PlusIcon, Trash2Icon } from "lucide-react";
import { Button } from "#/components/ui/button";
import { Checkbox } from "#/components/ui/checkbox";
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
    type ContextFieldDraft,
    createContextFieldDraft,
} from "./template-form-state";

const contextTypeItems = [
    { label: "Short text", value: "short_text" },
    { label: "Long text", value: "long_text" },
    { label: "Boolean", value: "boolean" },
    { label: "Number", value: "number" },
    { label: "Discord message link", value: "discord_message_link" },
];

type ContextFieldsEditorProps = {
    fields: ContextFieldDraft[];
    onChange: (fields: ContextFieldDraft[]) => void;
};

export function ContextFieldsEditor({
    fields,
    onChange,
}: ContextFieldsEditorProps) {
    function update(id: string, patch: Partial<Omit<ContextFieldDraft, "id">>) {
        onChange(
            fields.map((field) =>
                field.id === id ? { ...field, ...patch } : field,
            ),
        );
    }

    return (
        <section className="flex flex-col gap-3">
            <div className="flex items-center justify-between gap-4">
                <div>
                    <h3 className="font-medium text-sm">Context fields</h3>
                    <p className="text-muted-foreground text-xs">
                        Information moderators provide and affected members can
                        see.
                    </p>
                </div>
                <Button
                    disabled={fields.length >= 10}
                    onClick={() =>
                        onChange([...fields, createContextFieldDraft()])
                    }
                    size="sm"
                    type="button"
                    variant="outline"
                >
                    <PlusIcon aria-hidden="true" />
                    Add field
                </Button>
            </div>
            {fields.map((field) => (
                <div
                    className="grid gap-3 rounded-xl border p-3 sm:grid-cols-2"
                    key={field.id}
                >
                    <Field name={`context_label_${field.id}`}>
                        <FieldLabel>Label</FieldLabel>
                        <Input
                            maxLength={100}
                            onChange={(event) =>
                                update(field.id, {
                                    label: event.currentTarget.value,
                                })
                            }
                            required
                            type="text"
                            value={field.label}
                        />
                    </Field>
                    <Field name={`context_key_${field.id}`}>
                        <FieldLabel>Key</FieldLabel>
                        <Input
                            maxLength={64}
                            onChange={(event) =>
                                update(field.id, {
                                    key: event.currentTarget.value,
                                })
                            }
                            pattern="[a-z0-9_-]{2,64}"
                            placeholder="message_link"
                            required
                            type="text"
                            value={field.key}
                        />
                    </Field>
                    <Field name={`context_type_${field.id}`}>
                        <FieldLabel>Type</FieldLabel>
                        <Select
                            items={contextTypeItems}
                            onValueChange={(value) =>
                                value
                                    ? update(field.id, { type: value })
                                    : undefined
                            }
                            value={field.type}
                        >
                            <SelectTrigger>
                                <SelectValue />
                            </SelectTrigger>
                            <SelectPopup>
                                {contextTypeItems.map((item) => (
                                    <SelectItem
                                        key={item.value}
                                        value={item.value}
                                    >
                                        {item.label}
                                    </SelectItem>
                                ))}
                            </SelectPopup>
                        </Select>
                    </Field>
                    <div className="flex items-end justify-between gap-3">
                        <label
                            className="flex min-h-8 items-center gap-2 text-sm"
                            htmlFor={`required-${field.id}`}
                        >
                            <Checkbox
                                checked={field.required}
                                id={`required-${field.id}`}
                                onCheckedChange={(checked) =>
                                    update(field.id, {
                                        required: checked === true,
                                    })
                                }
                            />
                            Required
                        </label>
                        <Button
                            aria-label={`Remove ${field.label || "context field"}`}
                            onClick={() =>
                                onChange(
                                    fields.filter(
                                        (item) => item.id !== field.id,
                                    ),
                                )
                            }
                            size="icon-sm"
                            type="button"
                            variant="ghost"
                        >
                            <Trash2Icon aria-hidden="true" />
                        </Button>
                    </div>
                </div>
            ))}
        </section>
    );
}
