BEGIN {
	max=0
}

/.*KEYCLOAK_CLIENTS_([0-9]*).*/ {
	keycloak_var = match($0, /KEYCLOAK_CLIENTS_([0-9]*)/, arr)
	i = match(keycloak_var, /[0-9]*/)
	if (i > max) {
		max = i
	}
}

END {
	print max+1
}
