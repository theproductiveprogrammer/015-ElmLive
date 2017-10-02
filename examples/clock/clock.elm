module Main exposing (..)

import Html exposing (Html, program)
import Svg exposing (circle, line, svg)
import Svg.Attributes exposing (..)
import Time exposing (Time, second)


main =
    program { init = init, view = view, update = update, subscriptions = subs }



-- MODEL


type alias Model =
    Time



-- VIEW


view : Model -> Html Msg
view model =
    let
        angle =
            turns (Time.inMinutes model)

        handX =
            toString (50 + 35 * cos angle)

        handY =
            toString (50 + 35 * sin angle)
    in
        svg [ viewBox "0 0 100 100", width "300px" ]
            [ circle [ cx "50", cy "50", r "45", fill "red" ] []
            , circle [ cx "50", cy "50", r "40", fill "none", stroke "white" ] []
            , circle [ cx "50", cy "50", r "2", fill "black" ] []
            , circle [ cx handX, cy handY, r "1", fill "black" ] []
            , line [ x1 "50", y1 "50", x2 handX, y2 handY, stroke "black" ] []
            ]



-- UPDATE


type Msg
    = Tick Time


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        Tick newTime ->
            ( newTime, Cmd.none )


init : ( Model, Cmd Msg )
init =
    ( 0, Cmd.none )


subs : Model -> Sub Msg
subs model =
    Time.every second Tick
