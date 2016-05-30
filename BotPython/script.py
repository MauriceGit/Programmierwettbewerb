#!/usr/bin/python
# -*- coding: utf-8 -*-

import sys
import math
import random
import time

def calcDist(pos1, pos2):
    v = (pos1[0]-pos2[0], pos1[1]-pos2[1])
    return math.sqrt(v[0]*v[0] + v[1]*v[1])

def setRandomPos(myPos, longtermTarget, tryTargetTime):
    if int(time.time()-1 >= tryTargetTime) or calcDist(longtermTarget, myPos) < 10:
        NewlongtermTarget = (random.randint(0,1000), random.randint(0,1000))
        NewtryTargetTime = int(time.time())
        return NewlongtermTarget, NewtryTargetTime

    return longtermTarget, tryTargetTime

def findNearestToxin(myPos, toxins):
    dist = -1
    target = None
    for t in toxins:
        tPos = t[0]
        newDist = calcDist(myPos, tPos)
        if dist < 0 or newDist < dist:
            dist = newDist
            target = tPos
            #sys.stderr.write("Bot: " + myId + " is going for some food\n")
    return target

def findNearestFood(myPos, foods):
    dist = -1
    target = None
    for f in foods:
        fPos = f[0]
        newDist = calcDist(myPos, fPos)
        if dist < 0 or newDist < dist:
            dist = newDist
            target = fPos
            #sys.stderr.write("Bot: " + myId + " is going for some food\n")
    return target

def findNearestSmallerBlob(myPos, myMass, enemies):
    dist = -1
    target = None
    for e in enemies:
        ePos  = e[3]
        eMass = e[4]
        newDist = calcDist(myPos, ePos)
        if 0.99*myMass >= eMass and (dist < 0 or newDist < dist):
            dist = newDist
            target = ePos
            #sys.stderr.write("Bot: " + myId + " is now hunting\n")
    return target

def findAvgFleeingPos(myPos, myMass, enemies):
    target = None
    target = (0,0)
    seeEnemies = False
    for e in enemies:
        seeEnemies = True
        ePos  = e[3]
        eMass = e[4]

        if myMass < 0.9*eMass:
            target = (target[0] + myPos[0]-ePos[0], target[1] + myPos[1]-ePos[1])
        else:
            target = (target[0] + ePos[0]-myPos[0], target[1] + ePos[1]-myPos[1])

    target = (myPos[0] + target[0], myPos[1] + target[1])

    if not seeEnemies:
        target = None

    return target

def setFleeing(fleeing, fleeingTime, enemies, myMass):

    for e in enemies:
        if myMass < 0.9*e[4]:
            return True, int(time.time())

    if int(time.time())-1 > fleeingTime:
        return False, fleeingTime

    return fleeing, fleeingTime

def setHunting(enemies, myMass):
    for e in enemies:
        if myMass*0.9 > e[4]:
            #sys.stderr.write("Bot: " + myId + " is now hunting\n")
            return True
    return False


########################################################################
########################################################################
########################################################################

longtermTarget = (random.randint(0,1000), random.randint(0,1000))
target = longtermTarget
tryTargetTime = int(time.time())-5

fleeing = False
fleeingTime = int(time.time())-5

throwingTime = int(time.time())-5

# toxin = (Position, Mass)
# food  = (Position, Mass)
# blob  = (BotId, TeamId, Index, Position, Mass)
# data  = ([Eigene Blobs], [Alle anderen Blobs], [Alle Foods], [Alle Toxins])
while (1):
    sysIn      = sys.stdin.readline()
    #sys.stderr.write(sysIn + "\n")
    data = eval(sysIn)
    myBlobs = data[0]
    enemies = data[1]
    foods   = data[2]
    toxins  = data[3]

    # init
    myPos   = myBlobs[0][3]
    myMass  = 0
    myId    = str(myBlobs[0][0])
    throwNow = False

    # We let the smallest blob decide, where to go!
    for i in myBlobs:
        if i[4] < myMass or myMass == 0:
            myPos = i[3]
            myMass = i[4]

    # New random target every few seconds or when we reach it.
    longtermTarget, tryTargetTime = setRandomPos(myPos, longtermTarget, tryTargetTime)
    fleeing, fleeingTime = setFleeing(fleeing, fleeingTime, enemies, myMass)
    hunting = setHunting(enemies, myMass)

    # New fleeing position always overrides the normal target!
    # It also overrides fleeing from another enemy!
    if True and fleeing:
        tmpTarget = findAvgFleeingPos(myPos, myMass, enemies)
        if tmpTarget != None:
            target = tmpTarget

    else:
        # otherwise: hunting always overrides food and stuff.
        if True and hunting:
            target = findNearestSmallerBlob(myPos, myMass, enemies)
        else:
            toxinPos = findNearestToxin(myPos, toxins)
            if True and toxinPos != None and myMass > 200 and int(time.time())-1 > throwingTime:
                dist = calcDist(myPos, toxinPos)
                target = toxinPos
                if dist < 50 and dist > 20:
                    throwNow = True
                    throwingTime = int(time.time())
            else:
                foodPos = findNearestFood(myPos, foods)
                randPos = longtermTarget

                if foodPos != None:
                    target = foodPos
                else:
                    target = randPos

    # Time for some action?
    action = "None"
    if random.randint(0, 100) == 0:
        action = "split"
    if throwNow:
        action = "throw"

    #if random.randint(0,400) > 1:
    print "(%s,%s)" % (action, str(target))
    #else:
    #    print "hallo? Was jetzt"


    sys.stdout.flush();

    #exit(0)
