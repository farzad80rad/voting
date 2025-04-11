import http from 'k6/http';
import {group, check, sleep } from 'k6';
import encoding from 'k6/encoding';

const BASE_URL = 'http://localhost:80/api/v1';
const ADMIN_TOKEN = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySUQiOiI5ODMxMDI0Iiwicm9sZSI6ImFkbWluIiwiZXhwIjoxNzQ0NDEwODE3fQ.EiWdT-Bhiu07hlzPbsZ8cr1_RRiBMncNCRWwmbB4GBo'; // Put your real admin JWT here

const electionName = 'test-election';
const candidateCount = 5;
const totalVoters = 2000;


// IDs (set in setup)
let electionID;
let candidateIDs = [];
let voterTokens = [];


function generateVoterRequests(totalVoters) {
    const createRequests = [];
    const loginRequests = [];

    for (let i = 0; i < totalVoters; i++) {
        const userID = `user${i}`;

        // Create voter
        createRequests.push({
            method: 'POST',
            url: `${BASE_URL}/voter`,
            body: JSON.stringify({ userID }),
            params: {
                headers: {
                    Authorization: ADMIN_TOKEN,
                    'Content-Type': 'application/json'
                }
            }
        });

        // Prepare login request
        loginRequests.push({
            method: 'POST',
            url: `${BASE_URL}/authenticate`,
            body: JSON.stringify({
                username: userID,
                password: userID,
                role: "user"
            }),
            params: {
                headers: {
                    'Content-Type': 'application/json'
                }
            }
        });
    }

    return { createRequests, loginRequests };
}

export function setup() {
    // 1. Create election
    const electionRes = http.post(`${BASE_URL}/election`, JSON.stringify({
        electionName,
        startDate: "2006-01-02 15:04:05",
        endDate: "2026-05-02 15:04:05"
    }), {
        headers: {
            Authorization: ADMIN_TOKEN,
            'Content-Type': 'application/json'
        }
    });

    check(electionRes, {
        'Election created': (res) => res.status === 201
    });

    console.log(electionRes.body)

    electionID = JSON.parse(electionRes.body);

    // 2. Create candidates
    for (let i = 0; i < candidateCount; i++) {
        const id = `user${i}`;
        const res = http.post(`${BASE_URL}/candidate`, JSON.stringify({
            name: `candidate.${i}`,
            userID: id,
            electionID
        }), {
            headers: {
                Authorization: ADMIN_TOKEN,
                'Content-Type': 'application/json'
            }
        });

        candidateIDs.push(`candidate.${id}`);
    }

// Batch voter creation
    const { createRequests, loginRequests } = generateVoterRequests(totalVoters);
    const createResponses = http.batch(createRequests);


    // Batch logins
    const loginResponses = http.batch(loginRequests);
    const voterTokens = loginResponses.map(res => JSON.parse(res.body));

    return {
        electionID,
        candidateIDs,
        voterTokens
    };
}

export let options = {
    vus: totalVoters,
    iterations: totalVoters ,
    thresholds: {
        'http_req_failed{type:vote}': ['rate<0.05'],
    },
};

export default function (data) {
    const userIndex = __VU - 1 % data.voterTokens.length;
    const token = data.voterTokens[userIndex];
    const candidateID = data.candidateIDs[userIndex % data.candidateIDs.length];

    group('voting', function () {
        const res = http.post(`${BASE_URL}/vote`, JSON.stringify({
            candidateID,
            electionID: data.electionID
        }), {
            headers: {
                Authorization: token,
                'Content-Type': 'application/json'
            }
        });

        check(res, {
            'Vote success or already voted': (r) => r.status === 200 || r.status === 401 || r.status === 409
        });

    });

}
