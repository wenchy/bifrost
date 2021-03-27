#!/usr/bin/bash
########################################################
### @brief    start/stop switch for daemon app
### @depend
### @author   wenchyzhu(wenchyzhu@gmail.com)
### @date     2021-03-04
########################################################
function Usage()
{
    echo "Usage: $0 <start|stop|restart> [app]"
    echo "       -conf: config file path. (default "conf.yaml")"
}

APP='bifrost'
OPERATION='stop'
CONFPATH="conf.yaml"
PIDFILE=".pid"
# parse params
if [ $# -eq 1 ]; then
    OPERATION="$1"
elif [ $# -ge 2 ]; then
    OPERATION="$1"
    APP="$2"
else
    Usage
    exit -1
fi

function Start()
{
    cd $(dirname $0)
    echo "starting..."

    nohup ./${APP} -conf=${CONFPATH} > nohup.log 2>&1 &
    echo $! > ${PIDFILE}
    sleep 2 # sleep for 5 seconds, and then check if process has started successfully
    PID=`cat ${PIDFILE}`
    eval $(ps -ejf | awk -v PID=$PID -F" " '{if ($2 == PID) printf("PGID=%s", $4)}')
    if [[ $PGID == "" ]]; then
        echo "PID: $PID, PGID: not found"
        echo "Start Failed"
        exit -1
    else
        echo "PID: $PID, PGID: $PGID"
        echo "Start Success"
    fi
}

function Stop()
{
    cd $(dirname $0)
    # kill process id, only for tornado single-process mod
    # kill -TERM `cat ${PIDFILE}`

    # kill process group id, for both tornado multi-process and single-process mode
    PID=`cat ${PIDFILE}`
    eval $(ps -ejf | awk -v PID=$PID -F" " '{if ($2 == PID) printf("PGID=%s", $4)}')
    if [[ $PGID == "" ]]; then
        echo "PID: $PID, PGID: not found"
        echo "Stop Failed"
    else
        echo "PID: $PID, PGID: $PGID"
        kill -TERM -- -$PGID
        # sleep 3 # sleep for 5 seconds
        echo "Stop Success"
    fi
}

# avoid inappropriate Language Configuration
export LC_ALL="en_US.UTF-8"

case ${OPERATION} in
    start)
        Start
        ;;
    stop)
        Stop
        ;;
    restart)
        Stop
        Start
        ;;
    *)
        echo "invalid args!"
        Usage
        exit -1
esac