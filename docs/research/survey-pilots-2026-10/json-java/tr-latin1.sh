#!/bin/sh
exec java -Duser.language=tr -Duser.country=TR -Dfile.encoding=ISO-8859-1 -cp /tmp/jj/classes Driver "$@"
