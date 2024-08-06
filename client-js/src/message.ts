// Message

"use strict";

/**
 * Websocket message
 */
export interface WebsocketMessage {
    type: string;
    args?: { [key: string]: string };
    body?: string;
}

/**
 * Parses message
 * @param msg Input string
 * @returns Parsed message
 */
export function parseMessage(msg: string): WebsocketMessage {
    const lines = (msg + "").split("\n");
    const type = (lines[0] + "").trim().toUpperCase();
    const args = Object.create(null);
    let body = "";

    for (let i = 1; i < lines.length; i++) {
        if (lines[i].trim().length === 0) {
            body = lines.slice(i + 1).join("\n");
            break;
        }
        const argTxt = lines[i].split(":");
        const key = (argTxt[0] + "").trim().toLowerCase();
        const value = argTxt.slice(1).join(":").trim();
        args[key] = value;
    }

    return {
        type: type,
        args: args,
        body: body,
    };
}

/**
 * Serializes message
 * @param msg Message
 * @returns Serialized message as string
 */
export function makeMessage(msg: WebsocketMessage): string {
    let txt = "" + msg.type;

    if (msg.args) {
        for (const key in msg.args) {
            txt += "\n" + key + ": " + msg.args[key];
        }
    }

    if (msg.body) {
        txt += "\n\n" + (msg.body || "");
    }

    return txt;
}

/**
 * Get parameter from a parsed message
 * @param msg The parsed message
 * @param key The parameter key
 * @returns The parameter value
 */
export function getMessageParam(msg: WebsocketMessage, key: string): string {
    if (!msg.args) {
        return "";
    }

    return msg.args[key.toLowerCase()] || "";
}
