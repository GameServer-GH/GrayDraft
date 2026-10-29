#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../protos"
buf lint
buf generate
