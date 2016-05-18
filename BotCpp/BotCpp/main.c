#define PWB_IMPLEMENTATION
#include "pwb.h"

#include <time.h>
clock_t applicationStartTime = clock();

enum Task {
    SearchFood = 0,
    Flee
};

typedef BotCommand(*ProcessFunc)(double, VisibleGameState*);

struct State {
    Task task;
    ProcessFunc processFunc;

    FoodsOrToxinsArray knownFoods;

    Vec2 target = mkVec2(300, 300);

    Vec2 estimatedFieldSizeMin = mkVec2(FLT_MAX, FLT_MAX);
    Vec2 estimatedFieldSizeMax = mkVec2(FLT_MIN, FLT_MIN);

    double lastDirectionChange = clock();

    // ...
} state;

BotCommand searchFood(double t, VisibleGameState* visibleGameState) {
    //
    // Estimate field size
    //
    {
        for (int i = 0; i < visibleGameState->ownBlobs.numData; ++i) {
            Blob* blob = &visibleGameState->ownBlobs.data[i];
            if (blob->position.x < state.estimatedFieldSizeMin.x) {
                state.estimatedFieldSizeMin.x = blob->position.x;
            }
            if (blob->position.y < state.estimatedFieldSizeMin.y) {
                state.estimatedFieldSizeMin.y = blob->position.y;
            }
            if (blob->position.x > state.estimatedFieldSizeMax.x) {
                state.estimatedFieldSizeMax.x = blob->position.x;
            }
            if (blob->position.y > state.estimatedFieldSizeMax.y) {
                state.estimatedFieldSizeMax.y = blob->position.y;
            }
        }
    }

    //
    // My masses
    //
    float smallestMass = FLT_MAX;
    float biggestMass = FLT_MIN;
    {
        for (int i = 0; i < visibleGameState->ownBlobs.numData; ++i) {
            Blob* blob = &visibleGameState->ownBlobs.data[i];
            if (blob->mass < smallestMass) {
                smallestMass = blob->mass;
            }
            if (blob->mass > biggestMass) {
                biggestMass = blob->mass;
            }
        }
    }

    //
    // Own position
    //
    Vec2 myPosition = mkVec2(500, 500);
    {
        if (visibleGameState->ownBlobs.numData > 0) {
            Vec2 center = mkVec2(0, 0);
            for (int i = 0; i < visibleGameState->ownBlobs.numData; ++i) {
                center = addvv(&center, &visibleGameState->ownBlobs.data[i].position);
            }
            myPosition = divvf(&center, (float)visibleGameState->ownBlobs.numData);
        }
    }
    
    //
    // Target
    //
    {
        bool foodVisible = visibleGameState->foods.numData > 0;
        bool foeVisible = visibleGameState->otherBlobs.numData > 0;

        // Find nearest food
        FoodOrToxin* nearestFood = NULL;
        {
            float minDistance = FLT_MAX;
            for (int i = 0; i < visibleGameState->foods.numData; ++i) {
                FoodOrToxin* food = &visibleGameState->foods.data[i];
                Vec2 toFood = subvv(&food->position, &myPosition);
                float distance = length(&toFood);
                if (distance < minDistance) {
                    minDistance = distance;
                    nearestFood = food;
                }
            }
        }

#define EATING_DIFFERENCE 0.9f

        // Find a smaller foe
        Blob* smallerFoe = NULL;
        if (foeVisible) {           
            for (int i = 0; i < visibleGameState->otherBlobs.numData; ++i) {
                Blob* foe = &visibleGameState->otherBlobs.data[i];
                if (foe->mass < EATING_DIFFERENCE*smallestMass) {
                    smallerFoe = foe;
                }
            }
        }

        // Find a bigger foe
        Blob* biggerFoe = NULL;
        if (foeVisible) {
            for (int i = 0; i < visibleGameState->otherBlobs.numData; ++i) {
                Blob* foe = &visibleGameState->otherBlobs.data[i];
                if (EATING_DIFFERENCE*foe->mass > biggestMass) {
                    biggerFoe = foe;
                }
            }
        }

        if (biggerFoe != NULL) {
            Vec2 toBiggerFoe = subvv(&biggerFoe->position, &myPosition);
            toBiggerFoe = normalize(&toBiggerFoe);

            Vec2 flee = mulvf(&toBiggerFoe, -100.0f);            
            state.target = addvv(&state.target, &flee);
            Vec2 randomDirection = mkVec2(1.0f - 2.0f*randomNormalizedFloat(), 1.0f - 2.0f*randomNormalizedFloat());
            randomDirection = mulfv(10, &randomDirection);
            state.target = addvv(&state.target, &randomDirection);

            if (nearestFood != NULL) {
                Vec2 toNearestFood = subvv(&nearestFood->position, &myPosition);
                toNearestFood = normalize(&toNearestFood);
                if (dot(&toNearestFood, &toBiggerFoe) > 0.5f) {
                    state.target = nearestFood->position;
                }
            }            
        } else if (smallerFoe != NULL) {
            state.target = smallerFoe->position;
        } else if (foodVisible) {
            state.target = nearestFood->position;
        } else {            
            if (t > state.lastDirectionChange + 2.0) {
                Vec2 estimatedFieldSize = subvv(&state.estimatedFieldSizeMax, &state.estimatedFieldSizeMin);
                state.target = mkVec2(
                    state.estimatedFieldSizeMin.x + estimatedFieldSize.x*randomNormalizedFloat(),
                    state.estimatedFieldSizeMin.y + estimatedFieldSize.y*randomNormalizedFloat());
                state.lastDirectionChange = t;
            }
        }
    }

    return pwb_mkBotCommand(BotActionType::BatNone, &state.target);
}

BotCommand process(VisibleGameState* const visibleGameState) {
    clock_t now = clock();
    double t = double(now - applicationStartTime) / CLOCKS_PER_SEC;
    return state.processFunc(t, visibleGameState);
}

int main() {

    state.task = SearchFood;
    state.processFunc = searchFood;


    #define INPUT_BUFFER_MAX_LENGTH (20*1024)
    #define OUTPUT_BUFFER_MAX_LENGTH 100

    char inputBuffer[INPUT_BUFFER_MAX_LENGTH];
    char outputBuffer[OUTPUT_BUFFER_MAX_LENGTH];

    for (;;) {
        pwb_getline(inputBuffer, INPUT_BUFFER_MAX_LENGTH);

        // TODO(henk): Sometimes the input is the empty string. But the middleware never sends an empty string.
        if (strlen(inputBuffer) <= 0) {
            continue;
        }

        ParseContext parseContext = pwb_mkContext(inputBuffer);

        VisibleGameState visibleGameState;
        if (!pwb_parseAll(&parseContext, &visibleGameState)) {
            pwb_printErrors(&parseContext, stderr);
            exit(1);
        }

        // TODO: Process the input.
        BotCommand botCommand = process(&visibleGameState);

        pwb_toString(outputBuffer, &botCommand);
        printf("%s\n", outputBuffer);
        fflush(stdout);
    }

    return 0;
}