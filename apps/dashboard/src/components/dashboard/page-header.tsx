import type { ReactNode } from "react";

type PageHeaderProps = {
    title: string;
    description: string;
    actions?: ReactNode;
};

export function PageHeader({ title, description, actions }: PageHeaderProps) {
    return (
        <header className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
            <div className="flex min-w-0 flex-col gap-1">
                <h1 className="font-heading font-semibold text-2xl tracking-tight">
                    {title}
                </h1>
                <p className="text-muted-foreground text-sm">{description}</p>
            </div>
            {actions ? (
                <div className="flex shrink-0 items-center gap-2">
                    {actions}
                </div>
            ) : null}
        </header>
    );
}
