package utils

var MSG_TEMPLATE_1ST = `Dear %s,

We have scheduled a notification to be sent 24 hours before the departure of the following flight. Please reply with Y if you want to receive these flight alerts or any flight schedule changes.
Reply N if you do not want to receive them.

Your friends, at DAISI Travel
===========================

Passenger List:
%s

Segments:
SegNo FlightNo Class From  To    Depart Date/Time  Arrive Date/Time  Status
%s`

var MSG_TEMPLATE = `Dear %s, 

This is a friendly reminder of your upcoming travel departure.

Passenger List:
%s

*Airline*: %s
*Flight Number*: %s
*From*:  %s %s
*To*:   %s %s  
*Departure Date/Time*:  %s
*Arrival Date/Time*:  %s

This information is taken from your booking itinerary and is subject to change.  Please consult directly with your airline for the latest departure information.  
Have a safe and pleasant flight!

Your friends, at DAISI Travel
===========================`
