#!/bin/bash

# Hier bitte den Aufruf eures Programmes eintragen!
# Das kann ein Skript sein oder der Start eines Interpreteres.
# Hier würde ein Python-Skript mit dem Namen "pwb2.py" ausgeführt werden.
# Da kann aber auch "java -jar javastuff.jar" stehen :)
#
# ==================== Bitte hier anpassen ! ===========================
program="python script.py"
# ======================================================================

# ==================== Ab hier nichts mehr ändern! =====================
# Erstellung von named pipes um StdIn und StdOut umzuleiten.

rm -f pIn pOut pErr
mkfifo pIn
mkfifo pOut
mkfifo pErr

# Umleiten der Ein- und Ausgabe in named Pipes
(eval "$program" < pIn 2> pErr 1> pOut) &

# StdOut des Programms wird auf StdOut ausgegeben
# Gleichzeitig wird die Pipe zum Schreiben offen gehalten
(cat pOut) &
(>&2 cat pErr) &
# Dies ist nur dafür da, die Pipe zum Lesen offen zu halten, während das
# Programm läuft!!
sleep 10000000 > pIn &

while true
do
    # Stdin wird weiter geleitet
    read -e x

    printf "%s\n" "$x" >> pIn
done

