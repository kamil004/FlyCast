#!/bin/sh
CONF=/etc/config/qpkg.conf
QPKG_NAME="FlyCast"
QPKG_ROOT=`/sbin/getcfg $QPKG_NAME Install_Path -f ${CONF}`
export QNAP_QPKG=$QPKG_NAME

export FLYCAST_HOME="${QPKG_ROOT}/shared"
export FLYCAST_WEBROOT="${QPKG_ROOT}/web"
export FLYCAST_PORT=8100

# Media libraries: one path per line in shared/config/libraries.txt.
# Falls back to /share/Multimedia when the file is absent.
LIBCONF="${FLYCAST_HOME}/config/libraries.txt"
if [ -f "$LIBCONF" ]; then
  FLYCAST_LIBRARY=$(grep -v '^[[:space:]]*$' "$LIBCONF" | tr '\n' ':')
  FLYCAST_LIBRARY=${FLYCAST_LIBRARY%:}
elif [ -d /share/Multimedia ]; then
  FLYCAST_LIBRARY="/share/Multimedia"
fi
export FLYCAST_LIBRARY

PIDFILE="${QPKG_ROOT}/shared/flycast.pid"
LOGFILE="${QPKG_ROOT}/shared/logs/flycast.log"

case "$1" in
  start)
    ENABLED=$(/sbin/getcfg $QPKG_NAME Enable -u -d FALSE -f $CONF)
    if [ "$ENABLED" != "TRUE" ]; then
      echo "$QPKG_NAME is disabled."
      exit 1
    fi

    if [ -f "$PIDFILE" ] && kill -0 `cat $PIDFILE` 2>/dev/null; then
      echo "$QPKG_NAME already running"
      exit 0
    fi

    /bin/mkdir -p "${QPKG_ROOT}/shared/config" "${QPKG_ROOT}/shared/db" \
      "${QPKG_ROOT}/shared/thumbs" "${QPKG_ROOT}/shared/logs" \
      "${QPKG_ROOT}/shared/cache"

    ARCH=$(/bin/uname -m)
    BIN="${QPKG_ROOT}/flycast"
    FFMPEG_BIN="${QPKG_ROOT}/ffmpeg_amd64"
    case "$ARCH" in
      x86_64) [ -x "${QPKG_ROOT}/flycast_amd64" ] && BIN="${QPKG_ROOT}/flycast_amd64" ;;
      aarch64|arm64)
        [ -x "${QPKG_ROOT}/flycast_arm64" ] && BIN="${QPKG_ROOT}/flycast_arm64"
        FFMPEG_BIN="${QPKG_ROOT}/ffmpeg_arm64"
        ;;
    esac
    # Expose the bundled static ffmpeg under a stable name.
    if [ -x "$FFMPEG_BIN" ]; then
      /bin/ln -sf "$FFMPEG_BIN" "${QPKG_ROOT}/ffmpeg"
    fi

    /bin/chmod +x "$BIN" 2>/dev/null
    cd "${QPKG_ROOT}"
    "$BIN" >> "$LOGFILE" 2>&1 &
    echo $! > "$PIDFILE"
    echo "$QPKG_NAME started"
    ;;

  stop)
    if [ -f "$PIDFILE" ]; then
      kill `cat $PIDFILE` 2>/dev/null
      /bin/rm -f "$PIDFILE"
    fi
    killall -9 flycast flycast_amd64 flycast_arm64 2>/dev/null
    echo "$QPKG_NAME stopped"
    ;;

  restart)
    $0 stop
    /bin/sleep 2
    $0 start
    ;;

  status)
    if [ -f "$PIDFILE" ] && kill -0 `cat $PIDFILE` 2>/dev/null; then
      echo "$QPKG_NAME is running"
    else
      echo "$QPKG_NAME is stopped"
    fi
    ;;

  *)
    echo "Usage: $0 {start|stop|restart|status}"
    exit 1
    ;;
esac
exit 0
