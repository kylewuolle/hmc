#!/usr/bin/env bash

# Copyright 2025
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -eu

RELEASE_FILE=${RELEASE_FILE:-templates/provider/kcm-templates/files/release.yaml}
TEMPLATE_DIR=${TEMPLATE_DIR:-templates/provider/kcm-templates/files/templates}
BASE_COMMIT=${BASE_COMMIT:-origin/main}
HEAD_COMMIT=${HEAD_COMMIT:-HEAD}

COMMITTED_CHANGED=$(git diff --name-only "$BASE_COMMIT"...$HEAD_COMMIT -- "$TEMPLATE_DIR")
TRACKED_CHANGED=$(git diff --name-only -- "$TEMPLATE_DIR")
UNTRACKED_CHANGED=$(git ls-files --others --exclude-standard "$TEMPLATE_DIR")
ALL_CHANGED=$(echo -e "$COMMITTED_CHANGED\n$TRACKED_CHANGED\n$UNTRACKED_CHANGED" \
  | sort -u | grep -E '\.ya?ml$' || true)

for file in $ALL_CHANGED; do
  [[ -f "$file" ]] || continue

  kind=$(${YQ} e '.kind' "$file")
  [[ "$kind" != "ProviderTemplate" ]] && continue

  new_name=$(${YQ} e '.metadata.name' "$file")
  chart_name=$(${YQ} e '.spec.helm.chartSpec.chart' "$file")

  if [[ "$chart_name" == "kcm" ]]; then
    current=$(${YQ} e '.spec.kcm.template' "$RELEASE_FILE")
    if [[ "$current" != "$new_name" ]]; then
      ${YQ} e -i ".spec.kcm.template = \"$new_name\"" "$RELEASE_FILE"
      echo "Updated spec.kcm.template → $new_name"
    fi
  elif [[ "$chart_name" == "cluster-api" ]]; then
    current=$(${YQ} e '.spec.capi.template' "$RELEASE_FILE")
    if [[ "$current" != "$new_name" ]]; then
      ${YQ} e -i ".spec.capi.template = \"$new_name\"" "$RELEASE_FILE"
      echo "Updated spec.capi.template → $new_name"
    fi
  else
    index=$(${YQ} e ".spec.providers[] | select(.name == \"$chart_name\") | key" "$RELEASE_FILE")
    if [[ -n "$index" && "$index" != "null" ]]; then
      current=$(${YQ} e ".spec.providers[$index].template" "$RELEASE_FILE")
      if [[ "$current" != "$new_name" ]]; then
        ${YQ} e -i ".spec.providers[$index].template = \"$new_name\"" "$RELEASE_FILE"
        echo "Updated provider $chart_name template → $new_name"
      fi
    else
      echo "No matching provider entry for $chart_name in release.yaml; skipping"
    fi
  fi
done

