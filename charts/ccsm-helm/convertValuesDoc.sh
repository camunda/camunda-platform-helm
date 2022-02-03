# !/bin/bash


# reads the documentation of the values file and prints them in a table format, so we can easier copy them to the README
grep "# " values.yaml | awk '{ORS=""; print "| | `"; $2=tolower(substr($2,0,1))substr($2,2); print $2 "` | "; $1=$2=""; $3=toupper(substr($3, 0, 1))substr($3,2); ORS="\n"; print $0 " | | "}'
