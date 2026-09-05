import { useMutation, useQueryClient } from "@tanstack/react-query";
import { ArrowDownIcon, ArrowUpIcon, PlusIcon, Trash2Icon } from "lucide-react";
import { useState } from "react";
import { Alert, AlertDescription, AlertTitle } from "#/components/ui/alert";
import { Button } from "#/components/ui/button";
import {
    Card,
    CardDescription,
    CardHeader,
    CardPanel,
    CardTitle,
} from "#/components/ui/card";
import { Field, FieldDescription, FieldLabel } from "#/components/ui/field";
import { Input } from "#/components/ui/input";
import {
    Select,
    SelectItem,
    SelectPopup,
    SelectTrigger,
    SelectValue,
} from "#/components/ui/select";
import { Switch } from "#/components/ui/switch";
import { updateAppealSettings } from "#/lib/api";
import type { AppealQuestion, AppealSettings } from "#/lib/api-types";

const questionTypes = [
    { label: "Short text", value: "short_text" },
    { label: "Long text", value: "long_text" },
    { label: "Yes or no", value: "boolean" },
] satisfies Array<{ label: string; value: AppealQuestion["type"] }>;

type AppealSettingsFormProps = {
    csrfToken: string;
    guildId: string;
    settings: AppealSettings;
};

export function AppealSettingsForm({
    csrfToken,
    guildId,
    settings,
}: AppealSettingsFormProps) {
    const queryClient = useQueryClient();
    const [questions, setQuestions] = useState(
        settings.questions
            .slice()
            .sort((left, right) => left.position - right.position),
    );
    const [saved, setSaved] = useState(false);
    const mutation = useMutation({
        mutationFn: () =>
            updateAppealSettings(
                guildId,
                questions.map((question, position) => ({
                    ...question,
                    position,
                    prompt: question.prompt.trim(),
                })),
                csrfToken,
            ),
        onMutate: () => setSaved(false),
        onSuccess: async () => {
            await queryClient.invalidateQueries({
                queryKey: ["guild", guildId, "appeal-settings"],
            });
            setSaved(true);
        },
    });

    function updateQuestion(id: string, update: Partial<AppealQuestion>): void {
        setQuestions((current) =>
            current.map((question) =>
                question.id === id ? { ...question, ...update } : question,
            ),
        );
        setSaved(false);
    }

    function moveQuestion(index: number, direction: -1 | 1): void {
        setQuestions((current) => {
            const destination = index + direction;
            if (destination < 0 || destination >= current.length) {
                return current;
            }
            const next = [...current];
            [next[index], next[destination]] = [next[destination], next[index]];
            return next;
        });
        setSaved(false);
    }

    return (
        <Card>
            <CardHeader>
                <CardTitle>Appeal form</CardTitle>
                <CardDescription>
                    Questions snapshotted when a member submits an appeal.
                </CardDescription>
            </CardHeader>
            <CardPanel>
                <form
                    className="flex flex-col gap-5"
                    onSubmit={(event) => {
                        event.preventDefault();
                        mutation.mutate();
                    }}
                >
                    <div className="flex flex-col gap-3">
                        {questions.map((question, index) => (
                            <div
                                className="grid gap-4 rounded-xl border p-4 lg:grid-cols-[minmax(0,1fr)_12rem_auto]"
                                key={question.id}
                            >
                                <Field name={`prompt-${question.id}`}>
                                    <FieldLabel>
                                        Question {index + 1}
                                    </FieldLabel>
                                    <Input
                                        maxLength={300}
                                        onChange={(event) =>
                                            updateQuestion(question.id, {
                                                prompt: event.target.value,
                                            })
                                        }
                                        required
                                        value={question.prompt}
                                    />
                                </Field>
                                <Field>
                                    <FieldLabel>Answer type</FieldLabel>
                                    <Select
                                        items={questionTypes}
                                        onValueChange={(value) =>
                                            updateQuestion(question.id, {
                                                type: value as AppealQuestion["type"],
                                            })
                                        }
                                        value={question.type}
                                    >
                                        <SelectTrigger>
                                            <SelectValue />
                                        </SelectTrigger>
                                        <SelectPopup>
                                            {questionTypes.map((type) => (
                                                <SelectItem
                                                    key={type.value}
                                                    value={type.value}
                                                >
                                                    {type.label}
                                                </SelectItem>
                                            ))}
                                        </SelectPopup>
                                    </Select>
                                </Field>
                                <div className="flex items-end justify-between gap-2 lg:justify-end">
                                    <label
                                        className="flex min-h-9 items-center gap-2 text-sm"
                                        htmlFor={`required-${question.id}`}
                                    >
                                        <Switch
                                            checked={question.required}
                                            id={`required-${question.id}`}
                                            onCheckedChange={(checked) =>
                                                updateQuestion(question.id, {
                                                    required: checked,
                                                })
                                            }
                                        />
                                        Required
                                    </label>
                                    <div className="flex items-center">
                                        <Button
                                            aria-label="Move question up"
                                            disabled={index === 0}
                                            onClick={() =>
                                                moveQuestion(index, -1)
                                            }
                                            size="icon-sm"
                                            type="button"
                                            variant="ghost"
                                        >
                                            <ArrowUpIcon aria-hidden="true" />
                                        </Button>
                                        <Button
                                            aria-label="Move question down"
                                            disabled={
                                                index === questions.length - 1
                                            }
                                            onClick={() =>
                                                moveQuestion(index, 1)
                                            }
                                            size="icon-sm"
                                            type="button"
                                            variant="ghost"
                                        >
                                            <ArrowDownIcon aria-hidden="true" />
                                        </Button>
                                        <Button
                                            aria-label="Remove question"
                                            disabled={questions.length === 1}
                                            onClick={() => {
                                                setQuestions((current) =>
                                                    current.filter(
                                                        (item) =>
                                                            item.id !==
                                                            question.id,
                                                    ),
                                                );
                                                setSaved(false);
                                            }}
                                            size="icon-sm"
                                            type="button"
                                            variant="ghost"
                                        >
                                            <Trash2Icon aria-hidden="true" />
                                        </Button>
                                    </div>
                                </div>
                            </div>
                        ))}
                    </div>
                    <FieldDescription>
                        Keep one to ten questions. Existing appeals retain the
                        form they were submitted with.
                    </FieldDescription>
                    {mutation.isError ? (
                        <Alert variant="error">
                            <AlertTitle>Appeal form was not saved</AlertTitle>
                            <AlertDescription>
                                {mutation.error.message}
                            </AlertDescription>
                        </Alert>
                    ) : null}
                    {saved ? (
                        <Alert variant="success">
                            <AlertTitle>Appeal form saved</AlertTitle>
                            <AlertDescription>
                                Future appeals will use these questions.
                            </AlertDescription>
                        </Alert>
                    ) : null}
                    <div className="flex flex-wrap justify-between gap-2">
                        <Button
                            disabled={questions.length >= 10}
                            onClick={() => {
                                setQuestions((current) => [
                                    ...current,
                                    {
                                        id: `question-${crypto.randomUUID()}`,
                                        position: current.length,
                                        prompt: "",
                                        required: false,
                                        type: "long_text",
                                    },
                                ]);
                                setSaved(false);
                            }}
                            type="button"
                            variant="outline"
                        >
                            <PlusIcon aria-hidden="true" />
                            Add question
                        </Button>
                        <Button
                            disabled={questions.some(
                                (question) => !question.prompt.trim(),
                            )}
                            loading={mutation.isPending}
                            type="submit"
                        >
                            Save appeal form
                        </Button>
                    </div>
                </form>
            </CardPanel>
        </Card>
    );
}
