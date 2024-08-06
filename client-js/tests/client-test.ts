// Test

"use strict";

require('dotenv').config();

import assert from "assert";
import { PRCClient, StartedRequest } from "../src";

describe("PRC javascript client", () => {
    let client: PRCClient

    before(() => {
        client = new PRCClient({
            url: process.env.SERVER_URL || "ws://localhost:8080",
            authToken: process.env.AUTH_TOKEN || "",
            onServerError: (code, reason) => {
                console.error("[SERVER ERROR] " + code + ": " + reason);
            },
            onConnectionError: (err) => {
                console.error(err);
            },
        });

        client.connect();
    });

    let startedRequests: StartedRequest[] = [];

    it('Start requests', async () => {
        const results = await Promise.all([
            client.startRequest("test-type-1", 2),
            client.startRequest("test-type-1", 2),
            client.startRequest("test-type-1", 2),
            client.startRequest("test-type-2", 2),
            client.startRequest("test-type-2", 2),
        ]);

        startedRequests = results;

        let limitedCount = 0;

        for (let i = 0; i < 3; i++) {
            if (results[i].limited) {
                limitedCount++;
            }
        }

        assert.equal(limitedCount, 1);

        limitedCount = 0;

        for (let i = 3; i < 5; i++) {
            if (results[i].limited) {
                limitedCount++;
            }
        }

        assert.equal(limitedCount, 0);
    });

    it('Count requests', async () => {
        const counts = await Promise.all([
            client.getRequestCount("test-type-1"),
            client.getRequestCount("test-type-2"),
        ]);

        assert.equal(counts[0], 2);
        assert.equal(counts[1], 2);
    });

    it('End requests', () => {
        startedRequests.forEach(sr => {
            client.endRequest(sr);
        });
    });

    it('Wait for count to reach 0', async () => {
        let done = false;

        while (!done) {
            const counts = await Promise.all([
                client.getRequestCount("test-type-1"),
                client.getRequestCount("test-type-2"),
            ]);

            done = counts[0] === 0 && counts[1] === 0;
        }
    });

    after(() => {
        client.close();
    });
});
