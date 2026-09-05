import { useQuery } from "@tanstack/react-query";
import { createFileRoute, Link } from "@tanstack/react-router";
import { BirdIcon, ServerIcon } from "lucide-react";
import { QueryError, QueryPending } from "#/components/dashboard/query-state";
import { Avatar, AvatarFallback, AvatarImage } from "#/components/ui/avatar";
import { Badge } from "#/components/ui/badge";
import { Button } from "#/components/ui/button";
import {
    Card,
    CardAction,
    CardDescription,
    CardHeader,
    CardTitle,
} from "#/components/ui/card";
import {
    Empty,
    EmptyDescription,
    EmptyHeader,
    EmptyMedia,
    EmptyTitle,
} from "#/components/ui/empty";
import { SignOutButton } from "#/features/auth/sign-out-button";
import { guildsQuery } from "#/lib/queries";

export const Route = createFileRoute("/")({ component: Home });

function Home() {
    const guilds = useQuery(guildsQuery);

    if (guilds.isPending) {
        return <QueryPending label="Loading servers" />;
    }

    if (guilds.isError) {
        return (
            <main className="mx-auto flex min-h-svh w-full max-w-2xl items-center px-6">
                <QueryError onRetry={() => guilds.refetch()} />
            </main>
        );
    }

    const availableGuilds = guilds.data.filter((guild) => guild.quack_in_guild);

    return (
        <main className="mx-auto flex min-h-svh w-full max-w-5xl flex-col gap-8 px-4 py-8 sm:px-6 sm:py-12">
            <header className="flex items-center justify-between gap-4">
                <div className="flex items-center gap-3">
                    <div className="flex size-9 items-center justify-center rounded-lg border bg-card shadow-xs/5">
                        <BirdIcon aria-hidden="true" />
                    </div>
                    <div>
                        <h1 className="font-heading font-semibold text-xl">
                            Quack
                        </h1>
                        <p className="text-muted-foreground text-sm">
                            Choose a server to manage.
                        </p>
                    </div>
                </div>
                <SignOutButton />
            </header>

            {availableGuilds.length === 0 ? (
                <Empty className="rounded-2xl border">
                    <EmptyMedia variant="icon">
                        <ServerIcon aria-hidden="true" />
                    </EmptyMedia>
                    <EmptyHeader>
                        <EmptyTitle>No servers available</EmptyTitle>
                        <EmptyDescription>
                            Quack is not installed in a Discord server you can
                            manage. If you received a case notification, use its
                            dashboard link to view your cases or appeal.
                        </EmptyDescription>
                    </EmptyHeader>
                </Empty>
            ) : (
                <section
                    aria-label="Available Discord servers"
                    className="grid gap-3 sm:grid-cols-2"
                >
                    {availableGuilds.map((guild) => (
                        <Card key={guild.discord_guild_id}>
                            <CardHeader>
                                <div className="flex min-w-0 items-center gap-3">
                                    <Avatar className="size-10 rounded-lg">
                                        <AvatarImage
                                            alt=""
                                            src={guild.icon_url || undefined}
                                        />
                                        <AvatarFallback className="rounded-lg">
                                            {guild.name
                                                .slice(0, 2)
                                                .toUpperCase()}
                                        </AvatarFallback>
                                    </Avatar>
                                    <div className="min-w-0">
                                        <div className="flex items-center gap-2">
                                            <CardTitle className="truncate text-base">
                                                {guild.quack_guild_name ??
                                                    guild.name}
                                            </CardTitle>
                                            <Badge variant="success">
                                                Connected
                                            </Badge>
                                        </div>
                                        <CardDescription className="mt-1">
                                            {guild.is_owner
                                                ? "Owner"
                                                : guild.is_administrator
                                                  ? "Administrator"
                                                  : guild.can_manage_guild
                                                    ? "Manage Server"
                                                    : "Moderator"}
                                        </CardDescription>
                                    </div>
                                </div>
                                <CardAction>
                                    <Button
                                        render={
                                            <Link
                                                params={{
                                                    guildId:
                                                        guild.discord_guild_id,
                                                }}
                                                to="/guilds/$guildId"
                                            />
                                        }
                                        size="sm"
                                        variant="outline"
                                    >
                                        Open
                                    </Button>
                                </CardAction>
                            </CardHeader>
                        </Card>
                    ))}
                </section>
            )}
        </main>
    );
}
