#!/usr/bin/env bash
# Pinned-v2025.2 YBC controller runner. yb_backup.py is not used.
set -Eeuo pipefail
die(){ echo "YBC runner: $*" >&2; exit 1; }
op=${1:-}; shift || true; kind=""; policy=/etc/eubl21; target=""
while (($#)); do case $1 in --kind=*) kind=${1#*=};; --policy=*) policy=${1#*=};; --target-controller=*) target=${1#*=};; *) die "unknown option $1";; esac; shift; done
for f in database controllerAddress objectStoreUri objectStoreEndpoint objectStoreRegion; do [[ -r $policy/$f ]] || die "missing policy/$f"; done
db=$(<$policy/database); controller=$(<$policy/controllerAddress); uri=$(<$policy/objectStoreUri); endpoint=$(<$policy/objectStoreEndpoint); region=$(<$policy/objectStoreRegion)
path=${uri#s3://}; bucket=${path%%/*}; root=${path#*/}; [[ $bucket != "$path" ]] || die 'objectStoreUri requires bucket/prefix'
tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT; aws=(aws --endpoint-url "$endpoint" --region "$region"); state="$root/control-plane/state.json"
if ! "${aws[@]}" s3 cp "s3://$bucket/$state" "$tmp/state.json" >/dev/null 2>&1; then echo '{"chains":[],"lastSuccessfulBackup":null}' >"$tmp/state.json"; fi
mkdir "$tmp/ybc"; tar -xzf /home/yugabyte/share/ybc-*-linux-x86_64.tar.gz -C "$tmp/ybc" --strip-components=1; cli="$tmp/ybc/bin/yb-controller-cli"
save(){ "${aws[@]}" s3 cp "$tmp/state.json" "s3://$bucket/$state" --only-show-errors; }
backup(){
  [[ $kind == full || $kind == incremental ]] || die 'backup needs --kind=full|incremental'; prev=''; chain="full-$(date -u +%Y%m%dT%H%M%SZ)"
  if [[ $kind == incremental ]]; then prev=$(jq -r '.lastSuccessfulBackup // empty' "$tmp/state.json"); [[ -n $prev ]] || die 'incremental blocked: no predecessor'; chain=$(jq -r --arg p "$prev" '.chains[]|select(.dir==$p)|.chain' "$tmp/state.json"); [[ -n $chain ]] || die 'predecessor not retained'; fi
  id="$kind-$(date -u +%Y%m%dT%H%M%SZ)"; dir="$root/backups/$chain/$id"
  args=(backup --server_address="$controller" --server_port=18018 --tserver_ip="$controller" --ns="$db" --ns_type=ysql --cloud_type=s3 --bucket="$bucket" --cloud_dir="$dir" --wait); [[ -z $prev ]] || args+=(--prev_cloud_dir="$prev")
  "$cli" "${args[@]}" | tee "$tmp/$id.log"
  jq --arg id "$id" --arg kind "$kind" --arg chain "$chain" --arg dir "$dir" --arg prev "$prev" '.chains += [{id:$id,kind:$kind,chain:$chain,dir:$dir,previous:($prev|select(length>0)),completedAt:(now|todate)}] | .lastSuccessfulBackup=$dir' "$tmp/state.json" >"$tmp/new.json"; mv "$tmp/new.json" "$tmp/state.json"; save
  printf 'eubl21_ybc_last_successful_%s_backup_timestamp_seconds %s\n' "$kind" "$(date +%s)" | tee "$tmp/$kind.prom" | "${aws[@]}" s3 cp - "s3://$bucket/$root/evidence/$kind.prom" --only-show-errors
}
restore(){ [[ -n $target ]] || die 'restore requires --target-controller=host'; dir=$(jq -r '.lastSuccessfulBackup // empty' "$tmp/state.json"); [[ -n $dir ]] || die 'no recovery point'; "$cli" restore --server_address="$target" --server_port=18018 --tserver_ip="$target" --ns="$db" --ns_type=ysql --cloud_type=s3 --bucket="$bucket" --cloud_dir="$dir" --wait; }
case $op in backup) backup;; restore) restore;; *) die 'usage: backup|restore';; esac
