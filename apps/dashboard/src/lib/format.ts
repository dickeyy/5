export function formatDateTime(value: string): string {
    return new Intl.DateTimeFormat(undefined, {
        dateStyle: "medium",
        timeStyle: "short",
    }).format(new Date(value));
}

export function formatIdentifier(value: string): string {
    if (value.length <= 16) {
        return value;
    }
    return `${value.slice(0, 8)}…${value.slice(-6)}`;
}

export function titleCase(value: string): string {
    return value
        .replaceAll(/[._-]+/g, " ")
        .replaceAll(/\b\w/g, (character) => character.toUpperCase());
}
