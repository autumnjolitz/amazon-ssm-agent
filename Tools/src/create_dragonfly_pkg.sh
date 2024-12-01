#!/usr/bin/env bash
set -e

ARCH=$1
VERSION="$(head -1 "${GO_SPACE}/RELEASENOTES.md")"

generate-metadata() {
  echo "
amazon-ssm-agent is enabled in /etc/rc.conf via amazon_ssm_agent_enable=\"YES\"

it will automatically quit if it detects it is outside an AWS environment.
" | >&2 tee "$STAGEDIR/+DISPLAY"
  >&2 echo "Wrote package installtion message at ${STAGEDIR}/+DISPLAY"
  echo "
name = amazon-ssm-agent
version = ${VERSION}
origin = sysutils/amazon-ssm-plugin
comment = Amazon SSM agent
maintainer = autumn.jolitz+dragonfly-aws-ssm@gmail.com
www: https://github.com/autumnjolitz/amazon-ssm-agent
prefix: $PREFIX
desc = <<EOD
$(cat "${GO_SPACE}/README.md")

$(awk -v 'RS=' 'NR==1' "${GO_SPACE}/RELEASENOTES.md")
EOD
" | >&2 tee "$STAGEDIR/+MANIFEST"
  >&2 echo "Wrote package manifest at ${STAGEDIR}/+MANIFEST"
  echo "$STAGEDIR/"
}

generate-plist() {
  pushd "${STAGEDIR}" > /dev/null
  rm -rf plist || true
  >&2 echo 'List of files for package:'
  find . -type f -print | grep -vE '^(./)?(plist$|\+)' | sed 's|^\.||g' | >&2 tee plist
  local plistpath="$(pwd)/plist"
  popd >/dev/null
  echo "${plistpath}"
}

create-pkg () {
  local metadata="${1:-}"
  [ "x$metadata" = 'x' -o ! -d "$metadata" ] && >&2 echo 'No metadata given!' && return 1
  local plist="${2:-}"
  [ "x$plist" = 'x' -o ! -f "$plist" ] && >&2 echo 'No plist given!' && return 1
  local pkgdir="$GO_SPACE/bin/dragonfly_$ARCH/pkg/"
  rm -rf "$pkgdir" || true
  pkg create \
    -m "${metadata}" \
    -r "${STAGEDIR}/" \
    -p "$plist" \
    -o "$pkgdir"
  local pkgfile="$(echo $pkgdir*.pkg)"
  >&2 echo "You may 'sudo pkg add ${pkgfile}'"
  echo "$pkgfile"
}

echo "****************************************"
echo "Creating pkg file for DragonFly $ARCH"
echo "****************************************"


echo "Creating dragonfly folders dragonfly_$ARCH"
BUILD="${GO_SPACE}/bin/dragonfly_${ARCH}${DEBUG_FLAG}"
STAGEDIR="${BUILD}/dragonfly"
echo "Cleaning ${STAGEDIR}"
rm -rf "${STAGEDIR}" || true

PREFIX="${PREFIX:-/usr}"
[ "x$PREFIX" = 'x' ] && >&2 echo '$PREFIX empty!' && exit 2

if [ "x$PREFIX" = 'x/' -o "x$PREFIX" = 'x/usr' ]; then 
  if [ "x${EXEC_PREFIX:-}" = 'x' ]; then
    EXEC_PREFIX=/usr
  fi
  if [ "x$SYSCONFDIR" = 'x' ]; then
    SYSCONFDIR="/etc"
  fi
  if [ "x$DOCDIR" = 'x' ]; then
    DOCDIR=/usr/share/doc
  fi
  if [ "x$LOCALSTATEDIR" = 'x' ]; then
    LOCALSTATEDIR=/var
  fi
fi

DOCDIR="${DOCDIR:-"$PREFIX/share/doc"}"
EXEC_PREFIX="${EXEC_PREFIX:-$PREFIX}"
BINDIR="$EXEC_PREFIX/bin"
SYSCONFDIR="${SYSCONFDIR:-"$PREFIX/etc"}"
LOCALSTATEDIR="${LOCALSTATEDIR:-"$PREFIX/var"}"
mkdir -p "$STAGEDIR/$BINDIR"
mkdir -p "$STAGEDIR/$EXEC_PREFIX"
mkdir -p "$STAGEDIR/$SYSCONFDIR/init/"
mkdir -p "$STAGEDIR/$SYSCONFDIR/amazon/ssm/"
mkdir -p "$STAGEDIR/$SYSCONFDIR/rc.d"
mkdir -p "$STAGEDIR/$DOCDIR/amazon-ssm-agent/"
mkdir -p "$STAGEDIR/$LOCALSTATEDIR/lib/amazon/ssm/"

echo "Copying application files dragonfly_$ARCH"

cp ${GO_SPACE}/packaging/dragonfly/+* $STAGEDIR/.
cp ${GO_SPACE}/packaging/dragonfly/rc.d/amazon-ssm-agent $STAGEDIR/$SYSCONFDIR/rc.d/.

sed -i '' 's|$BINDIR|'"${BINDIR}"'|g' $STAGEDIR/$SYSCONFDIR/rc.d/amazon-ssm-agent

cp ${BUILD}/amazon-ssm-agent $STAGEDIR/$BINDIR/.
cp ${BUILD}/ssm-agent-worker $STAGEDIR/$BINDIR/.
cp ${BUILD}/ssm-cli $STAGEDIR/$BINDIR/.
cp ${BUILD}/ssm-document-worker $STAGEDIR/$BINDIR/.
cp ${BUILD}/ssm-session-worker $STAGEDIR/$BINDIR/.
cp ${BUILD}/ssm-session-logger $STAGEDIR/$BINDIR/.
cp ${GO_SPACE}/seelog_unix.xml $STAGEDIR/$SYSCONFDIR/amazon/ssm/seelog.xml.template
cp ${GO_SPACE}/amazon-ssm-agent.json.template $STAGEDIR/$SYSCONFDIR/amazon/ssm/
# cp ${GO_SPACE}/packaging/ubuntu/amazon-ssm-agent.conf $STAGEDIR/$SYSCONFDIR/init/
# cp ${GO_SPACE}/packaging/ubuntu/amazon-ssm-agent.service $STAGEDIR/lib/systemd/system/

echo "Copying dragonfly package config files dragonfly_$ARCH"

cp ${GO_SPACE}/RELEASENOTES.md $STAGEDIR/$SYSCONFDIR/amazon/ssm/RELEASENOTES.md
cp ${GO_SPACE}/README.md $STAGEDIR/$SYSCONFDIR/amazon/ssm/README.md
cp ${GO_SPACE}/NOTICE.md $STAGEDIR/$SYSCONFDIR/amazon/ssm/
cp ${GO_SPACE}/Tools/src/LICENSE $STAGEDIR/$DOCDIR/amazon-ssm-agent/copyright


create-pkg "$(generate-metadata)" "$(generate-plist)"


