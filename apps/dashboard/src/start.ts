import { createStart } from "@tanstack/react-start";

// The API owns authentication and its session cookie is resolved in the browser.
// Rendering routes only on the client keeps that boundary consistent across
// same-origin and cross-origin dashboard deployments.
export const startInstance = createStart(() => ({
    defaultSsr: false,
}));
