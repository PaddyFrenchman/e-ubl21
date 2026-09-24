#!/usr/bin/env bash
# YBC backup control-plane. Credentials are intentionally obtained only from
# the workload identity credential chain understood by aws(1); no credential
# flags, files, ConfigMaps, or evidence are accepted by this program.
set -Eeuo pipefail

die() { echo "eubl21-ybc-control-plane: $*" >&2; exit 1; }
now() { date -u +%Y-%m-%dT%H:%M:%SZ; }
sha() { sha256sum "$1" | awk '{print $1}'; }

command_name="${1:-}"; shift || true
kind=""; policy_dir="/etc/eubl21"; restore_target=""
while (($#)); do
  case "$1" in
    --kind=*) kind="${1#*=}";; --policy=*) policy_dir="${1#*=}";;
    --target-masters=*) restore_target="${1#*=}";; *) die "unknown argument: $1";;
  esac
  shift
done
[[ -r "$policy_dir/database" && -r "$policy_dir/masterAddresses" && -r "$policy_dir/objectStoreUri" ]] || die "policy is incomplete"
database="$(<"$policy_dir/database")"; masters="$(<"$policy_dir/masterAddresses")"
store_uri="$(<"$policy_dir/objectStoreUri")"; endpoint="$(<"$policy_dir/objectStoreEndpoint")"
region="$(<"$policy_dir/objectStoreRegion")"; retention="$(<"$policy_dir/retention")"
[[ "$store_uri" =~ ^s3://[^/]+/.+ ]] || die "objectStoreUri must be s3://bucket/prefix"
bucket_and_prefix="${store_uri#s3://}"; bucket="${bucket_and_prefix%%/*}"; prefix="${bucket_and_prefix#*/}"
aws_args=(--region "$region" --endpoint-url "$endpoint")
state_key="$prefix/control-plane/state.json"; evidence_prefix="$prefix/evidence"
work="$(mktemp -d)"; trap 'rm -rf "$work"' EXIT
get_state() {
  if ! aws "${aws_args[@]}" s3 cp "s3://$bucket/$state_key" "$work/state.json" >/dev/null 2>&1; then
    printf '%s\n' '{"chains":[],"lastSuccessfulFull":null,"lastSuccessfulIncremental":null}' >"$work/state.json"
  fi
}
put_state() { aws "${aws_args[@]}" s3 cp "$work/state.json" "s3://$bucket/$state_key" --only-show-errors; }
metric() { printf 'eubl21_ybc_last_successful_backup_timestamp_seconds{kind="%s"} %s\n' "$1" "$(date +%s)"; }
backup() {
  [[ "$kind" == full || "$kind" == incremental ]] || die "backup requires --kind=full or --kind=incremental"
  get_state
  local parent="" chain id location result manifest started completed
  started="$(now)"
  if [[ "$kind" == incremental ]]; then
    parent="$(jq -r '.lastSuccessfulFull // empty' "$work/state.json")"
    [[ -n "$parent" ]] || die "incremental backup is blocked: no successful full chain exists"
    chain="$(jq -r --arg id "$parent" '.chains[] | select(.id==$id and .status=="succeeded") | .chain' "$work/state.json")"
    [[ -n "$chain" ]] || die "incremental backup is blocked: successful full chain is unavailable"
  else chain="full-$(date -u +%Y%m%dT%H%M%SZ)"; fi
  id="$kind-$(date -u +%Y%m%dT%H%M%SZ)-${HOSTNAME:-runner}"; location="$store_uri/backups/$chain/$id"
  # --use_ybc makes this a Controller backup. ysql_dump is never invoked.
  result="$work/$id.log"
  /home/yugabyte/bin/yb_backup.py create --use_ybc --masters "$masters" --keyspace "$database" --backup_location "$location" ${parent:+--incremental_backup --backup_id "$parent"} >"$result" 2>&1
  completed="$(now)"; manifest="$work/$id.json"
  jq -n --arg id "$id" --arg kind "$kind" --arg chain "$chain" --arg parent "$parent" --arg location "$location" --arg started "$started" --arg completed "$completed" --arg masters "$masters" \
    '{id:$id,kind:$kind,chain:$chain,parent:($parent|select(length>0)),location:$location,startedAt:$started,completedAt:$completed,sourceMasters:$masters,status:"succeeded"}' >"$manifest"
  jq --argjson backup "$(cat "$manifest")" --arg kind "$kind" --arg id "$id" '.chains += [$backup] | if $kind=="full" then .lastSuccessfulFull=$id else .lastSuccessfulIncremental=$id end' "$work/state.json" >"$work/new.json"
  mv "$work/new.json" "$work/state.json"; put_state
  aws "${aws_args[@]}" s3 cp "$manifest" "s3://$bucket/$evidence_prefix/$id.json" --only-show-errors
  aws "${aws_args[@]}" s3 cp "$result" "s3://$bucket/$evidence_prefix/$id.log" --only-show-errors
  metric "$kind" | aws "${aws_args[@]}" s3 cp - "s3://$bucket/$evidence_prefix/metrics.prom" --only-show-errors
  echo "YBC $kind backup succeeded: id=$id chain=$chain location=$location manifest_sha256=$(sha "$manifest")"
}
enforce_retention() {
  get_state
  # Policy is a jq object, e.g. {"keepFull":35,"keepIncremental":140}.
  [[ "$retention" =~ ^\{ ]] || die "retention must be JSON with keepFull and keepIncremental"
  local keep_full keep_incremental
  keep_full="$(jq -r '.keepFull' <<<"$retention")"; keep_incremental="$(jq -r '.keepIncremental' <<<"$retention")"
  jq -c --argjson full "$keep_full" --argjson inc "$keep_incremental" '[.chains | sort_by(.completedAt) | reverse | group_by(.kind)[] | .[if .[0].kind=="full" then $full else $inc end:][]] | .[]' "$work/state.json" | while IFS= read -r old; do
    location="$(jq -r .location <<<"$old")"; aws "${aws_args[@]}" s3 rm "$location" --recursive --only-show-errors
  done
  jq --argjson full "$keep_full" --argjson inc "$keep_incremental" '.chains |= (sort_by(.completedAt) | reverse | group_by(.kind) | map(if .[0].kind=="full" then .[:$full] else .[:$inc] end) | flatten)' "$work/state.json" >"$work/new.json"; mv "$work/new.json" "$work/state.json"; put_state
}
restore() {
  [[ -n "$restore_target" ]] || die "restore requires --target-masters=host:7100,... (an isolated clean target)"
  get_state; id="$(jq -r '.lastSuccessfulIncremental // .lastSuccessfulFull // empty' "$work/state.json")"; [[ -n "$id" ]] || die "no retained recovery point"
  location="$(jq -r --arg id "$id" '.chains[] | select(.id==$id) | .location' "$work/state.json")"
  /home/yugabyte/bin/yb_backup.py restore --use_ybc --masters "$restore_target" --keyspace "$database" --backup_location "$location"
  jq -n --arg id "$id" --arg target "$restore_target" --arg completed "$(now)" '{recoveryPoint:$id,targetMasters:$target,completedAt:$completed,status:"succeeded"}' >"$work/restore.json"
  aws "${aws_args[@]}" s3 cp "$work/restore.json" "s3://$bucket/$evidence_prefix/restore-$id.json" --only-show-errors
}
case "$command_name" in backup) backup;; retention) enforce_retention;; restore) restore;; *) die 'usage: backup|retention|restore';; esac
