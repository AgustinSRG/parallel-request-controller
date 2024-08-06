// Utils

"use strict";

/**
 * Struct to store a pending request details
 */
export interface PendingRequest {
    // Request type
    requestType: string;

    // Limit for the number of parallel requests
    limit: number;
}

/**
 * Promise listener
 */
export interface PromiseListener<T> {
    // The promise
    promise: Promise<T>;

    // True if resolved
    resolved: boolean;

    // Timeout
    timeout?: NodeJS.Timeout;

    // On resolve
    onResolve?: (t: T) => void;

    // On reject
    onReject?: (err: Error) => void;

    // Value
    t?: T;

    // Error
    error?: Error;
}

/**
 * Creates promise listener
 * @param timeout The timeout (milliseconds)
 * @returns The promise listener
 */
export function createPromiseListener<T>(timeout: number): PromiseListener<T> {
    const res: PromiseListener<T> = {
        resolved: false,
        promise: null,
    };

    res.promise = new Promise<T>((resolve, reject) => {
        res.onResolve = resolve;
        res.onReject = reject;

        if (res.resolved) {
            if (res.error) {
                reject(res.error);
            } else {
                resolve(res.t);
            }
        }
    });

    res.timeout = setTimeout(() => {
        res.timeout = undefined;

        if (res.resolved) {
            return;
        }

        res.resolved = true;
        
        if (res.onReject) {
            res.onReject(new Error("Timeout"));
        } else {
            res.error = new Error("Timeout");
        }

    }, timeout);

    return res;
}

/**
 * Resolves promise listener
 * @param listener The listener
 * @param val The value
 */
export function resolvePromiseListener<T>(listener: PromiseListener<T>, val: T) {
    if (listener.resolved) {
        return;
    }

    if (listener.timeout) {
        clearTimeout(listener.timeout);
        listener.timeout = undefined;
    }

    if (listener.onResolve) {
        listener.onResolve(val);
    } else {
        listener.t = val;
    }

    listener.resolved = true;
}
