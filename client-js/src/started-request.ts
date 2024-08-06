// Started request

"use strict";

/**
 * Required data to keep in order to indicate the request ending
 */
export interface StartedRequest {
    // Request ID
    id: number;

    // True if limited
    limited: boolean;

    // Connection index
    connectionIndex?: number;
}
