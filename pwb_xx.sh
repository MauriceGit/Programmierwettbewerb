#!/bin/bash

# Hier bitte den Aufruf eures Programmes eintragen!
# Das kann ein Skript sein oder der Start eines Interpreteres.
# Hier würde ein Python-Skript mit dem Namen "pwb2.py" ausgeführt werden.
# Da kann aber auch "java -jar javastuff.jar" stehen :)
#
# ==================== Bitte hier anpassen ! ===========================
program="pwb2.py"
# ======================================================================

# ==================== Ab hier nichts mehr ändern! =====================
# Erstellung von named pipes um StdIn und StdOut umzuleiten.
if [[ ! -p pIn ]]; then
    mkfifo pIn
fi

if [[ ! -p pOut ]]; then
    mkfifo pOut
fi

# Umleiten der Ein- und Ausgabe in named Pipes
(./"$program" < pIn > pOut) &

# StdOut des Programms wird auf StdOut ausgegeben
cat pOut &

while true
do
    # Stdin wird weiter geleitet
    read x
    printf "%s\n" "$x" >> pIn
done

