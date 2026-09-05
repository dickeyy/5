import { AlertCircleIcon } from "lucide-react";
import { Alert, AlertDescription, AlertTitle } from "#/components/ui/alert";
import { Button } from "#/components/ui/button";
import { Spinner } from "#/components/ui/spinner";

export function QueryPending({ label = "Loading" }: { label?: string }) {
    return (
        <div className="flex min-h-52 items-center justify-center">
            <Spinner />
            <span className="sr-only">{label}</span>
        </div>
    );
}

export function QueryError({
    description = "Quack could not load this data. Try again in a moment.",
    onRetry,
}: {
    description?: string;
    onRetry: () => void;
}) {
    return (
        <Alert variant="error">
            <AlertCircleIcon aria-hidden="true" />
            <AlertTitle>Something went wrong</AlertTitle>
            <AlertDescription>
                <p>{description}</p>
                <div>
                    <Button onClick={onRetry} type="button" variant="outline">
                        Try again
                    </Button>
                </div>
            </AlertDescription>
        </Alert>
    );
}
