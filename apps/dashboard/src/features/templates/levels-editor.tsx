import { PlusIcon, Trash2Icon } from "lucide-react";
import { Button } from "#/components/ui/button";
import { Checkbox } from "#/components/ui/checkbox";
import { Field, FieldDescription, FieldLabel } from "#/components/ui/field";
import { Input } from "#/components/ui/input";
import {
    Select,
    SelectItem,
    SelectPopup,
    SelectTrigger,
    SelectValue,
} from "#/components/ui/select";
import { createLevelDraft, type LevelDraft } from "./template-form-state";

const actionItems = [
    { label: "Create case only", value: "none" },
    { label: "Timeout member", value: "timeout_user" },
    { label: "Kick member", value: "kick_user" },
    { label: "Ban member", value: "ban_user" },
];

type LevelsEditorProps = {
    levels: LevelDraft[];
    onChange: (levels: LevelDraft[]) => void;
};

export function LevelsEditor({ levels, onChange }: LevelsEditorProps) {
    function update(id: string, patch: Partial<Omit<LevelDraft, "id">>) {
        onChange(
            levels.map((level) =>
                level.id === id ? { ...level, ...patch } : level,
            ),
        );
    }

    return (
        <section className="flex flex-col gap-3">
            <div className="flex items-center justify-between gap-4">
                <div>
                    <h3 className="font-medium text-sm">Escalation levels</h3>
                    <p className="text-muted-foreground text-xs">
                        The default applies first. Higher levels start at their
                        configured case count.
                    </p>
                </div>
                <Button
                    onClick={() =>
                        onChange([...levels, createLevelDraft(false)])
                    }
                    size="sm"
                    type="button"
                    variant="outline"
                >
                    <PlusIcon aria-hidden="true" />
                    Add level
                </Button>
            </div>
            {levels.map((level) => (
                <div
                    className="flex flex-col gap-4 rounded-xl border p-3"
                    key={level.id}
                >
                    <div className="grid gap-3 sm:grid-cols-2">
                        <Field name={`level_name_${level.id}`}>
                            <FieldLabel>Level name</FieldLabel>
                            <Input
                                onChange={(event) =>
                                    update(level.id, {
                                        name: event.currentTarget.value,
                                    })
                                }
                                required
                                type="text"
                                value={level.name}
                            />
                        </Field>
                        {level.isDefault ? (
                            <Field>
                                <FieldLabel>Starts at</FieldLabel>
                                <Input disabled type="text" value="Default" />
                            </Field>
                        ) : (
                            <Field name={`trigger_${level.id}`}>
                                <FieldLabel>Starts at case</FieldLabel>
                                <Input
                                    min={1}
                                    onChange={(event) =>
                                        update(level.id, {
                                            triggerCaseCount: Number(
                                                event.currentTarget.value,
                                            ),
                                        })
                                    }
                                    required
                                    type="number"
                                    value={level.triggerCaseCount}
                                />
                            </Field>
                        )}
                        <Field name={`action_${level.id}`}>
                            <FieldLabel>Action</FieldLabel>
                            <Select
                                items={actionItems}
                                onValueChange={(value) =>
                                    value
                                        ? update(level.id, {
                                              actionType: value,
                                          })
                                        : undefined
                                }
                                value={level.actionType}
                            >
                                <SelectTrigger>
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectPopup>
                                    {actionItems.map((item) => (
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
                        {level.actionType === "timeout_user" ? (
                            <Field name={`timeout_${level.id}`}>
                                <FieldLabel>Timeout seconds</FieldLabel>
                                <Input
                                    max={2_419_200}
                                    min={1}
                                    onChange={(event) =>
                                        update(level.id, {
                                            timeoutDurationSeconds: Number(
                                                event.currentTarget.value,
                                            ),
                                        })
                                    }
                                    required
                                    type="number"
                                    value={level.timeoutDurationSeconds}
                                />
                                <FieldDescription>
                                    Maximum 28 days (2,419,200 seconds).
                                </FieldDescription>
                            </Field>
                        ) : null}
                        {level.actionType === "ban_user" ? (
                            <Field name={`delete_messages_${level.id}`}>
                                <FieldLabel>Message history seconds</FieldLabel>
                                <Input
                                    max={604_800}
                                    min={0}
                                    onChange={(event) =>
                                        update(level.id, {
                                            deleteMessageSeconds: Number(
                                                event.currentTarget.value,
                                            ),
                                        })
                                    }
                                    required
                                    type="number"
                                    value={level.deleteMessageSeconds}
                                />
                                <FieldDescription>
                                    Use 0 to keep message history.
                                </FieldDescription>
                            </Field>
                        ) : null}
                        {level.actionType !== "none" ? (
                            <Field name={`retries_${level.id}`}>
                                <FieldLabel>Safe retries</FieldLabel>
                                <Input
                                    max={10}
                                    min={0}
                                    onChange={(event) =>
                                        update(level.id, {
                                            maxRetries: Number(
                                                event.currentTarget.value,
                                            ),
                                        })
                                    }
                                    required
                                    type="number"
                                    value={level.maxRetries}
                                />
                            </Field>
                        ) : null}
                    </div>
                    <div className="flex items-center justify-between gap-4">
                        <label
                            className="flex items-center gap-2 text-sm"
                            htmlFor={`notify-${level.id}`}
                        >
                            <Checkbox
                                checked={level.notifyUser}
                                id={`notify-${level.id}`}
                                onCheckedChange={(checked) =>
                                    update(level.id, {
                                        notifyUser: checked === true,
                                    })
                                }
                            />
                            Notify member
                        </label>
                        {!level.isDefault ? (
                            <Button
                                onClick={() =>
                                    onChange(
                                        levels.filter(
                                            (item) => item.id !== level.id,
                                        ),
                                    )
                                }
                                size="sm"
                                type="button"
                                variant="ghost"
                            >
                                <Trash2Icon aria-hidden="true" />
                                Remove level
                            </Button>
                        ) : null}
                    </div>
                </div>
            ))}
        </section>
    );
}
