#!/bin/sh
# Mock AI agent script for testing recommend.Engine.
# Outputs a Claude-style JSON envelope with structured_output.
cat <<'EOF'
{"type":"result","subtype":"success","result":"","session_id":"test-session","structured_output":{"recommendations":[{"name":"architect-adviser","kind":"skill","source":"github:owner/catalog","confidence":0.92,"reason":"Project uses hexagonal architecture patterns"},{"name":"react","kind":"rule","source":"github:owner/catalog","confidence":0.85,"reason":"React detected in package.json"},{"name":"low-confidence-item","kind":"skill","source":"github:owner/catalog","confidence":0.45,"reason":"Weakly related item below default threshold"}]}}
EOF
