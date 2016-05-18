#ifndef PWB_HEADER
#define PWB_HEADER

/*
----------------------------------------------------------------------------------------------
General
----------------------------------------------------------------------------------------------

This library contains:

    - a parser to parse the input to the bot which comes from the server via
      the middleware and

    - a few functions to work with 2d vectors.

----------------------------------------------------------------------------------------------
Usage
----------------------------------------------------------------------------------------------

This file contains the header AND the implementation of the pwb-library. When you 
need only the header, you can simply include it with:

    > #include "pwb.h"

When you want to create the implementation, you have to define the macro "PWB_IMPLEMENTATION":

    > #define PWB_IMPLEMENTATION
    > #include "pwb.h"

Do this only once in your program! Otherwise you will have multiple definitions on functions
and the compiler will yell at you!

----------------------------------------------------------------------------------------------
Example
----------------------------------------------------------------------------------------------

BotCommand process(const VisibleGameState* const visibleGameState) {
    Vec2 target;
    BotActionType botActionType;

    // TODO: Calculate the target and action of your bot!

    return pwb_mkBotCommand(botActionType, &target);
}

int main() {
    #define INPUT_BUFFER_MAX_LENGTH 2000
    #define OUTPUT_BUFFER_MAX_LENGTH 100

    char inputBuffer[INPUT_BUFFER_MAX_LENGTH];
    char outputBuffer[OUTPUT_BUFFER_MAX_LENGTH];

    for (;;) {
        pwb_getline(inputBuffer, INPUT_BUFFER_MAX_LENGTH);

        ParseContext parseContext = pwb_mkContext(inputBuffer);

        VisibleGameState visibleGameState;
        if (!pwb_parseAll(&parseContext, &visibleGameState)) {
            pwb_printErrors(&parseContext, stderr);
            exit(1);
        }

        BotCommand botCommand = process(&visibleGameState);

        pwb_toString(outputBuffer, &botCommand);
        printf("%s\n", outputBuffer);
        fflush(stdout);
    }

    return 0;
}

----------------------------------------------------------------------------------------------
Example explanation
----------------------------------------------------------------------------------------------

You can implement the behaviour of your bot in the "process"-function. 

    > BotCommand process(const VisibleGameState* const visibleGameState) { ... }

"visibleGameState" contains the state of the game that is currently visible to your bot. This 
information comes from the server via the middleware. After you've done your calculations you 
have to return a BotCommand, which is subsequently sent back to the server via the middleware:

You can create the botCommand by using the "pwb_mkBotCommand"-function:

    > pwb_mkBotCommand(botActionType, &target);

To read the string which is sent to your bot by the middleware from stdin, you can use the 
"pwb_getline"-function:

    > pwb_getline(inputBuffer, INPUT_BUFFER_MAX_LENGTH);

(The library does not use any dynamic-memory! Just allocate a sufficient block when your bot
is started and reuse that every frame.)

To parse the input, you have to create a "ParseContext". That can be done by calling the 
"pwb_mkContext"-function. Simply provide a pointer to the input string:

    > ParseContext parseContext = pwb_mkContext(inputBuffer);

Call the "pwb_parseAll"-function to initiate the parsing:

    > VisibleGameState visibleGameState;
    > if (!pwb_parseAll(&parseContext, &visibleGameState)) { ... }

When true is returned, the parsing went well and you can call your "process"-function. 
Otherwise you can print the error-stack by calling the "pwb_printErrors"-function:

    > pwb_printErrors(stderr, &parseContext);

Send your computed "BotCommand" back to the server (via the middleware) by calling:

    > pwb_toString(outputBuffer, &botCommand);
    > printf("%s\n", outputBuffer);
    > fflush(stdout);

Here, again, you have to provide the memory beforehand. Ensure that the buffer is big enough.

----------------------------------------------------------------------------------------------
Further reading
----------------------------------------------------------------------------------------------

Further down, you can find the function-declarations with additional documentation.

*/

#ifdef _MSC_VER
#define _CRT_SECURE_NO_WARNINGS
#endif

#include <iostream>
#include <string>
#include <cmath>
#include <float.h>
#include <array>

#define MIN(X, Y) (((X) < (Y)) ? (X) : (Y))
#define MAX(X, Y) (((X) > (Y)) ? (X) : (Y))

#define NUM_MAX_ERRORS 10
#define NUM_MAX_PARSE_LEVELS 10

#define PI 3.1415926535f;

int floatEqual(float lhs, float rhs, float epsilon);
float randomNormalizedFloat();
float clamp(float x, float min, float max);

struct Vec2 {
    float x, y;
};

Vec2 mkVec2(float x, float y) {
    Vec2 v;
    v.x = x;
    v.y = y;
    return v;
}

Vec2 addvv(const Vec2* const lhs, const Vec2* const rhs);
Vec2 subvv(const Vec2* const lhs, const Vec2* const rhs);
Vec2 subvf(const Vec2* const v);
Vec2 mulvf(const Vec2* const v, float f);
Vec2 mulfv(float f, const Vec2* const v);
Vec2 mulvv(const Vec2* const lhs, const Vec2* const rhs);
Vec2 divvf(const Vec2* const v, float f);
Vec2 divvv(const Vec2* const lhs, const Vec2* const rhs);
float dot(const Vec2* const lhs, const Vec2* const rhs);
float lengthSquared(const Vec2* const v);
float length(const Vec2* const v);
float distanceSquared(const Vec2* const lhs, const Vec2* const rhs);
float distance(const Vec2* const lhs, const Vec2* const rhs);
float minNorm(const Vec2* const v);
float maxNorm(const Vec2* const v);
Vec2 normalize(const Vec2* const v);
bool equal(const Vec2* const lhs, const Vec2* const rhs);
void copy(Vec2* const dst, const Vec2* const src);
void swap(Vec2* const lhs, Vec2* const rhs);
Vec2 componentWiseMin(const Vec2* const lhs, const Vec2* const rhs);
Vec2 componentWiseMax(const Vec2* const lhs, const Vec2* const rhs);
Vec2 componentWiseAbs(const Vec2* const v);

enum BotActionType { BatNone, BatThrow, BatSplit };
struct BotCommand {
    BotActionType botActionType;
    Vec2 target;
};

typedef struct Blob {
    int botId;
    int teamId;
    int blobId;
    Vec2 position;
    float mass;
} Blob;

typedef struct FoodOrToxin {
    Vec2 position;
    float mass;
} FoodOrToxin;

typedef struct BlobsArray {
    Blob data[2000];
    int numData;
} BlobsArray;

typedef struct FoodsOrToxinsArray {
    FoodOrToxin data[2000];
    int numData;
} FoodsOrToxinsArray;

typedef struct VisibleGameState {
    BlobsArray ownBlobs;
    BlobsArray otherBlobs;
    FoodsOrToxinsArray foods;
    FoodsOrToxinsArray toxins;
} VisibleGameState;

typedef struct ParserInvocation {
    const char* name;
    const char* location;
    const char* errorMessage;
} ParserInvocation;

ParserInvocation mkParserInvokation(const char* name, const char* location) {
    ParserInvocation parserInvocation;
    parserInvocation.name = name;
    parserInvocation.location = location;
    parserInvocation.errorMessage = nullptr;
    return parserInvocation;
}

typedef struct ParseContext {
    const char* input;
    const char* next;
    bool success;
    ParserInvocation parserInvocations[NUM_MAX_PARSE_LEVELS];
    int numParserInvocations;
    int numMaxParserInvocations;
} ParseContext;

/**
 * Reads a whole line from stdin.
 *
 * @brief Reads characters from stdin until a '\n' is encountered and returns a pointer to the first
 *        charater.
 *
 * @param   buffer          A buffer for storing the characters. The library does not use dynamic
 *                          memory, so you have to provide the memory for the input. Overestimate
 *                          the potential length of the input by a large amount and everything
 *                          will be fine. Allocate the memory at the start of your program and
 *                          reuse it to speed things up.
 * @param   bufferLength    Size of "buffer".
 *
 * @result  Returns "buffer".
 */
char* pwb_getline(char* buffer, int bufferLength);

/**
 * Creates a new context for parsing.
 *
 * @brief The parser needs a "ParseContext" to track the parsing. This function lets you create a
 *        new "ParseContext".
 *
 * @param   input   Pointer to the string, that shall be parsed.
 *
 * @result  A newly created "ParseContext".
 */
ParseContext pwb_mkContext(const char* input);

/**
 * Parse the input to the bot.
 *
 * @brief Parses what the middleware sends to the bot and puts all the information into the 
 *        "VisibleGameState"-struct. When the input could not be parsed, true is returned. Otherwise,
 *        you can print the error by using the "pwb_printErrors" function.
 *
 * @param   parseContext            Information that is used for parsing. Create a new ParseContext
 *                                  by using the "pwb_mkContext" function.
 * @param   outVisibleGameState     Here, the current state of the game is stored. (Only what is
 *                                  visible to your bot of course.)
 * 
 * @result  true is returned, when the parsing went well. Otherwise false is returned.
 */
int pwb_parseAll(ParseContext* parseContext, VisibleGameState* outVisibleGameState);


/**
 * Prints the errors, that occoured while parsing on the given "ParseContext".
 *
 * @brief When parser did not succeed, you can use this function to print the error stack, that was
 *        recored by the ParseContext, to an arbirary stream or file.
 *
 * @param   parseContext    ParseContext that recored the errors.
 * @param   file            Stream or file to print the errors to.
 */
void pwb_printErrors(ParseContext* parseContext, FILE* file);

/**
 * Creates a new BotCommand.
 *
 * @brief Creates a new command that can be sent to the server.
 *
 * @param   botActionType   Action, the bot shall perform.
 * @param   target          Target of the bot.
 *
 * @return  BotCommand that has the action "botActionType" and the target "target".
 */
BotCommand pwb_mkBotCommand(BotActionType botActionType, const Vec2* const target);

/**
 * Writes the string representation of "botCommand" to the buffer.
 *
 * @brief The library does not allocate memory, so you have to provide enough memory to fit in the string representation
 *        of the "BotCommand".
 *
 * @param   buffer      Buffer for the string.
 * @param   botCommand  "BotCommand", that shall be written into the buffer.
 */
void pwb_toString(char* buffer, const BotCommand* const botCommand);

// ------------------------------------------------------------------------------------------------------------------------
#ifdef PWB_IMPLEMENTATION

typedef struct ParseResult {
    bool success;
    int advance;
    const char* next;
} ParseResult;


Vec2 mkUnitX = mkVec2(1, 0);
Vec2 mkUnitY = mkVec2(0, 1);
Vec2 mkOne = mkVec2(1, 1);
Vec2 mkZero = mkVec2(0, 0);

int floatEqual(float lhs, float rhs, float epsilon) { return abs(lhs - rhs) < epsilon; }
float randomNormalizedFloat() { return float(rand()) / float(RAND_MAX); }

Vec2 addvv(const Vec2* const lhs, const Vec2* const rhs) { return mkVec2(lhs->x + rhs->x, lhs->y + rhs->y); }
Vec2 subvv(const Vec2* const lhs, const Vec2* const rhs) { return mkVec2(lhs->x - rhs->x, lhs->y - rhs->y); }
Vec2 subvf(const Vec2* const v) { return mkVec2(-v->x, -v->y); }
Vec2 mulvf(const Vec2* const v, float f) { return mkVec2(v->x*f, v->y*f); }
Vec2 mulfv(float f, const Vec2* const v) { return mulvf(v, f); }
Vec2 mulvv(const Vec2* const lhs, const Vec2* const rhs) { return mkVec2(lhs->x*rhs->x, lhs->y*rhs->y); }
Vec2 divvf(const Vec2* const v, float f) { return mkVec2(v->x / f, v->y / f); };
Vec2 divvv(const Vec2* const lhs, const Vec2* const rhs) { return mkVec2(lhs->x / rhs->x, lhs->y / rhs->y); }
float dot(const Vec2* const lhs, const Vec2* const rhs) { return lhs->x*rhs->x + lhs->y*rhs->y; }
float lengthSquared(const Vec2* const v) { return dot(v, v); }
float length(const Vec2* const v) { return sqrt(lengthSquared(v)); }
float distanceSquared(const Vec2* const lhs, const Vec2* const rhs) { Vec2 diff = subvv(lhs, rhs); return lengthSquared(&diff); }
float distance(const Vec2* const lhs, const Vec2* const rhs) { Vec2 diff = subvv(lhs, rhs); return length(&diff); }
float minNorm(const Vec2* const v) { return MIN(v->x, v->y); }
float maxNorm(const Vec2* const v) { return MAX(v->x, v->y); }
Vec2 normalize(const Vec2* const v) { return divvf(v, length(v)); }
bool equal(const Vec2* const lhs, const Vec2* const rhs) { return abs(lhs->x - rhs->x) < FLT_EPSILON && abs(lhs->y - rhs->y) < FLT_EPSILON; }
void copy(Vec2* const dst, const Vec2* const src) { memcpy(dst, src, sizeof(Vec2)); }
void swap(Vec2* const lhs, Vec2* const rhs) { Vec2 tmp = *lhs; copy(lhs, rhs); copy(rhs, &tmp); }
Vec2 componentWiseMin(const Vec2* const lhs, const Vec2* const rhs) { return mkVec2(MIN(lhs->x, rhs->x), MIN(lhs->y, rhs->y)); }
Vec2 componentWiseMax(const Vec2* const lhs, const Vec2* const rhs) { return mkVec2(MAX(lhs->x, rhs->x), MAX(lhs->y, rhs->y)); }
Vec2 componentWiseAbs(const Vec2* const v) { return mkVec2(abs(v->x), abs(v->y)); }

void pwb_printErrors(ParseContext* parseContext, FILE* file) {
    if (!parseContext->success) {
        fprintf(stderr, "ERROR on input: \"%s\"\n", parseContext->input);
        for (int i = 0; i < parseContext->numMaxParserInvocations; ++i) {
            ParserInvocation* parserInvocation = &parseContext->parserInvocations[i];
            fprintf(file, "ERROR\n");
            if (parserInvocation->errorMessage != nullptr) {
                fprintf(file, "    Message:  %s\n", parserInvocation->errorMessage);
            }
            fprintf(file, "    Parser:   %s\n", parserInvocation->name);
            fprintf(file, "    Location: %s\n", parserInvocation->location);
        }
    }
}

BotCommand pwb_mkBotCommand(BotActionType botActionType, const Vec2* const target) {
    BotCommand botCommand;
    botCommand.botActionType = botActionType;
    botCommand.target = *target;
    return botCommand;
}

void pushLevel(ParseContext* const parseContext, const char* name) {
    parseContext->parserInvocations[parseContext->numParserInvocations] = mkParserInvokation(name, parseContext->next);
    ++parseContext->numParserInvocations;
    ++parseContext->numMaxParserInvocations;
}

void popLevel(ParseContext* const parseContext) {
    --parseContext->numParserInvocations;
    if (parseContext->success) {
        --parseContext->numMaxParserInvocations;
    }
}

ParseContext pwb_mkContext(const char* input) {
    ParseContext parseContext;
    parseContext.input = input;
    parseContext.next = input;
    parseContext.success = true;
    parseContext.numParserInvocations = 0;
    parseContext.numMaxParserInvocations = 0;
    return parseContext;
}

ParseContext mkContext(const ParseContext* const otherParseContext) {
    ParseContext parseContext;
    parseContext.input = otherParseContext->input;
    parseContext.next = otherParseContext->next;
    parseContext.success = otherParseContext->success;
    memcpy(parseContext.parserInvocations, otherParseContext->parserInvocations, NUM_MAX_PARSE_LEVELS*sizeof(const char*));
    parseContext.numParserInvocations = otherParseContext->numParserInvocations;
    parseContext.numMaxParserInvocations = otherParseContext->numMaxParserInvocations;
    return parseContext;
}

ParseResult mkError(ParseContext* const parseContext, const char* message) {
    ParserInvocation* parserInvocation = &parseContext->parserInvocations[parseContext->numParserInvocations];
    parserInvocation->errorMessage = message;

    parseContext->success = false;

    ParseResult parseResult;
    parseResult.success = false;
    parseResult.advance = 0;
    parseResult.next = parseContext->next;

    popLevel(parseContext);

    return parseResult;
}

ParseResult mkResult(ParseContext* const parseContext, void* value, int advance) {
    auto next = parseContext->next + advance;

    parseContext->next = next;

    ParseResult parseResult;
    parseResult.success = true;
    parseResult.advance = advance;
    parseResult.next = next + advance;

    popLevel(parseContext);

    return parseResult;
}

ParseResult parseChar(ParseContext* const parseContext, char c, bool* outHasChar) {
    pushLevel(parseContext, __FUNCTION__);

    if (*parseContext->next == c) {
        if (outHasChar != nullptr) {
            *outHasChar = true;
        }
        return mkResult(parseContext, outHasChar, 1);
    }
    return mkError(parseContext, "Could not find the char.");
}

ParseResult parseFloat(ParseContext* const parseContext, float* outFloat) {
    pushLevel(parseContext, __FUNCTION__);

    int advance;
    int numAssignments = sscanf(parseContext->next, "%f%n", outFloat, &advance);
    if (numAssignments > 0) {
        return mkResult(parseContext, outFloat, advance);
    }
    return mkError(parseContext, "Could not parse the float.");
}

ParseResult parseInt(ParseContext* const parseContext, int* outInteger) {
    pushLevel(parseContext, __FUNCTION__);

    int advance;
    int numAssignments = sscanf(parseContext->next, "%i%n", outInteger, &advance);
    if (numAssignments > 0) {
        return mkResult(parseContext, outInteger, advance);
    }
    return mkError(parseContext, "Could not parse the integer.");
}

ParseResult parseWhiteSpaces(ParseContext* const parseContext) {
    pushLevel(parseContext, __FUNCTION__);

    int advance = 0;
    const char* runner = parseContext->next;
    while (*runner == ' ' || *runner == '\t') {
        ++advance;
        ++runner;
    }
    return mkResult(parseContext, nullptr, advance);
}

ParseResult parseVec2(ParseContext* const parseContext, Vec2* outVec2) {
    pushLevel(parseContext, __FUNCTION__);

    parseWhiteSpaces(parseContext);
    if (!parseChar(parseContext, '(', nullptr).success) return mkError(parseContext, "Could not find the opening bracket.");
    parseWhiteSpaces(parseContext);
    if (!parseFloat(parseContext, &outVec2->x).success) return mkError(parseContext, "Could not find the x-value.");
    parseWhiteSpaces(parseContext);
    if (!parseChar(parseContext, ',', nullptr).success) return mkError(parseContext, "Could not find the comma.");
    parseWhiteSpaces(parseContext);
    if (!parseFloat(parseContext, &outVec2->y).success) return mkError(parseContext, "Could not find the y-value.");
    parseWhiteSpaces(parseContext);
    if (!parseChar(parseContext, ')', nullptr).success) return mkError(parseContext, "Could not find the closing bracket.");
    return mkResult(parseContext, outVec2, 0);
}

ParseResult parseBlob(ParseContext* const parseContext, Blob* outBlob) {
    pushLevel(parseContext, __FUNCTION__);

    parseWhiteSpaces(parseContext);    if (!parseChar(parseContext, '(', nullptr).success) return mkError(parseContext, "Could not find the opening bracket.");    parseWhiteSpaces(parseContext);
    if (!parseInt(parseContext, &outBlob->botId).success) return mkError(parseContext, "Could not find the botId.");
    parseWhiteSpaces(parseContext);
    if (!parseChar(parseContext, ',', nullptr).success) return mkError(parseContext, "Could not find the comma.");
    parseWhiteSpaces(parseContext);
    if (!parseInt(parseContext, &outBlob->teamId).success) return mkError(parseContext, "Could not find the teamId.");
    parseWhiteSpaces(parseContext);
    if (!parseChar(parseContext, ',', nullptr).success) return mkError(parseContext, "Could not find the comma.");
    parseWhiteSpaces(parseContext);
    if (!parseInt(parseContext, &outBlob->blobId).success) return mkError(parseContext, "Could not find the blobId.");
    parseWhiteSpaces(parseContext);
    if (!parseChar(parseContext, ',', nullptr).success) return mkError(parseContext, "Could not find the comma.");
    parseWhiteSpaces(parseContext);
    if (!parseVec2(parseContext, &outBlob->position).success) return mkError(parseContext, "Could not find the position of the blob.");
    parseWhiteSpaces(parseContext);
    if (!parseChar(parseContext, ',', nullptr).success) return mkError(parseContext, "Could not find the comma.");
    parseWhiteSpaces(parseContext);
    if (!parseFloat(parseContext, &outBlob->mass).success) return mkError(parseContext, "Could not find the mass of the bot.");
    parseWhiteSpaces(parseContext);
    if (!parseChar(parseContext, ')', nullptr).success) return mkError(parseContext, "Could not find the closing bracket.");
    return mkResult(parseContext, outBlob, 0);
}

ParseResult parseFoodOrToxin(ParseContext* const parseContext, FoodOrToxin* outFoodOrToxin) {
    pushLevel(parseContext, __FUNCTION__);

    parseWhiteSpaces(parseContext);
    if (!parseChar(parseContext, '(', nullptr).success) return mkError(parseContext, "Could not find the opening bracket.");
    parseWhiteSpaces(parseContext);
    if (!parseVec2(parseContext, &outFoodOrToxin->position).success) return mkError(parseContext, "Could not find the positon of the food.");
    parseWhiteSpaces(parseContext);
    if (!parseChar(parseContext, ',', nullptr).success) return mkError(parseContext, "Could not find the comma.");
    parseWhiteSpaces(parseContext);
    if (!parseFloat(parseContext, &outFoodOrToxin->mass).success) return mkError(parseContext, "Could not find the mass of the food.");
    parseWhiteSpaces(parseContext);
    if (!parseChar(parseContext, ')', nullptr).success) return mkError(parseContext, "Could not find the closing bracket.");
    return mkResult(parseContext, outFoodOrToxin, 0);
}

ParseResult parseBlobsList(ParseContext* const parseContext, BlobsArray* blobs) {
    pushLevel(parseContext, __FUNCTION__);

    blobs->numData = 0;

    parseWhiteSpaces(parseContext);

    if (!parseChar(parseContext, '[', nullptr).success) {
        return mkError(parseContext, "Could not find the opening bracket.");
    }

    parseWhiteSpaces(parseContext);

    auto emptyListContext = mkContext(parseContext);
    if (parseChar(&emptyListContext, ']', nullptr).success) {
        *parseContext = emptyListContext;
        return mkResult(parseContext, nullptr, 0);
    }

    bool somethingFollows = true;
    while (somethingFollows) {
        parseWhiteSpaces(parseContext);

        Blob blob;
        if (!parseBlob(parseContext, &blob).success) {
            return mkError(parseContext, "Could not parse the blob.");
        }
        blobs->data[blobs->numData] = blob;
        ++blobs->numData;

        parseWhiteSpaces(parseContext);

        if (!parseChar(parseContext, ',', &somethingFollows).success) {
            somethingFollows = false;
        }
    }

    if (!parseChar(parseContext, ']', nullptr).success) {
        return mkError(parseContext, "Could not find the closing bracket.");
    }

    return mkResult(parseContext, blobs, 0);
}

ParseResult parseFoodsOrToxinsList(ParseContext* const parseContext, FoodsOrToxinsArray* foodsOrToxins) {
    pushLevel(parseContext, __FUNCTION__);

    foodsOrToxins->numData = 0;

    parseWhiteSpaces(parseContext);

    if (!parseChar(parseContext, '[', nullptr).success) {
        return mkError(parseContext, "Could not find the opening bracket.");
    }

    auto emptyListContext = mkContext(parseContext);
    if (parseChar(&emptyListContext, ']', nullptr).success) {
        *parseContext = emptyListContext;
        return mkResult(parseContext, nullptr, 0);
    }

    bool somethingFollows = true;
    while (somethingFollows) {
        parseWhiteSpaces(parseContext);

        FoodOrToxin foodOrToxin;
        if (!parseFoodOrToxin(parseContext, &foodOrToxin).success) {
            return mkError(parseContext, "Could not parse the food.");
        }
        foodsOrToxins->data[foodsOrToxins->numData] = foodOrToxin;
        ++foodsOrToxins->numData;

        parseWhiteSpaces(parseContext);

        if (!parseChar(parseContext, ',', &somethingFollows).success) {
            somethingFollows = false;
        }
    }

    if (!parseChar(parseContext, ']', nullptr).success) {
        return mkError(parseContext, "Could not find the closing bracket.");
    }

    return mkResult(parseContext, foodsOrToxins, 0);
}

void pwb_toString(char* buffer, const BotCommand* const botCommand) {
    char botActionTypeBuffer[6];
    switch (botCommand->botActionType) {
    case BatNone:  sprintf(botActionTypeBuffer, "None");  break;
    case BatSplit: sprintf(botActionTypeBuffer, "Split"); break;
    case BatThrow: sprintf(botActionTypeBuffer, "Throw"); break;
    }
    sprintf(buffer, "(%s, (%.2f, %.2f))", botActionTypeBuffer, botCommand->target.x, botCommand->target.y);
}

ParseResult parseAll(ParseContext* parseContext, VisibleGameState* outVisibleGameState) {
    pushLevel(parseContext, __FUNCTION__);

    parseWhiteSpaces(parseContext);
    if (!parseChar(parseContext, '(', nullptr).success) { return mkError(parseContext, "Could not find the opening bracket."); }
    parseWhiteSpaces(parseContext);
    if (!parseBlobsList(parseContext, &outVisibleGameState->ownBlobs).success) { return mkError(parseContext, "Could not parse the own blobs."); }
    parseWhiteSpaces(parseContext);
    if (!parseChar(parseContext, ',', nullptr).success) return mkError(parseContext, "Could not find the comma.");
    parseWhiteSpaces(parseContext);
    if (!parseBlobsList(parseContext, &outVisibleGameState->otherBlobs).success) { return mkError(parseContext, "Could not find the other blobs."); }
    parseWhiteSpaces(parseContext);
    if (!parseChar(parseContext, ',', nullptr).success) return mkError(parseContext, "Could not find the comma.");
    parseWhiteSpaces(parseContext);
    if (!parseFoodsOrToxinsList(parseContext, &outVisibleGameState->foods).success) { return mkError(parseContext, "Could not find the foods."); }
    parseWhiteSpaces(parseContext);
    if (!parseChar(parseContext, ',', nullptr).success) return mkError(parseContext, "Could not find the comma.");
    parseWhiteSpaces(parseContext);
    if (!parseFoodsOrToxinsList(parseContext, &outVisibleGameState->toxins).success) { return mkError(parseContext, "Could not find the toxins."); }
    parseWhiteSpaces(parseContext);
    if (!parseChar(parseContext, ')', nullptr).success) { return mkError(parseContext, "Could not find the closing bracket."); }
    return mkResult(parseContext, outVisibleGameState, 0);
}

int pwb_parseAll(ParseContext* parseContext, VisibleGameState* outVisibleGameState) {
    return parseAll(parseContext, outVisibleGameState).success;
}

char* pwb_getline(char* buffer, int bufferLength) {
    int spaceLeft = bufferLength;
    int c;

    for (;;) {
        c = fgetc(stdin);
        if (c == EOF) {
            break;
        }

        if (--spaceLeft == 1) {
            break;
        }

        *buffer = c;
        ++buffer;
        if (c == '\n') {
            break;
        }
    }

    *buffer = '\0';
    return buffer;
}

    
#endif
#endif
