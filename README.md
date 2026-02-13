# Personal Price Tracker and Web Scrapper for Second Hand Listings

## Needed API Keys and Services

- Discord API key to add the Bot to your server
- OpenVPN or WireGaurd VPN for Basic Proxy implementation,
-- so your home IP doesn't get Blacklisted too fast
- Geopify API key,
  - this one is used to calculate the distance from Facebook
  - listings to decide weather to include them or not
- MongoDB API key
  - the DB isn't included in the docker compose so you're gonna have
  - to make an instance yourself, (maybe ill add it later)
  - everything else is handled tho, as long as it can make a connection

1. <https://www.geoapify.com/get-started-with-maps-api/>
2. <https://discord.com/developers/docs/quick-start/getting-started>
3. <https://www.mongodb.com/products/platform/atlas-database>
4. or self host your own MongoDB instance, has to be version 8.0 or higher
5. keep in mind in atlas free you can only make 3 channels per mongodb deployment

## Setup Guide

For deploying the but, just copy and paste the API keys into the docker compose
file and `docker compose pull && docker compose up`
You can also just compile the Golang binary using `go build .`
but the application needs a chrome runtime to function properly
(this is included in the docker image)
after installing the bot you have to call setup in the channels you want
the bot to message.
you will need your location code from a Facebook marketplace
<https://www.facebook.com/marketplace/107711145919004/search?maxPrice=754&query=Asus%20XG27AQDMG&exact=false>
this is the digits immediately following marketplace/ so in this case 107711145919004.
You also need the city and state in Los Angeles, CA format and the max distance for
how far the marketplace listings will be.
For the rest just read the Available Functions section below

## Available Functions

1. **add**: adds an item to be tracked
   - Required Inputs:
     - Name: item name used for regex in getting second hand queries
     - uri: scraping URI
     - html_tag: CSS query used to get the element containing the price
     - timer: how often the scraper function gets called (in hours)
     - type: item category
       - Tech
       - Clothes

2. **remove**: removes item from tracking list and the DB
   - Required Inputs:
     - Name: item name to remove

3. **list**: lists all tracked items

4. **channel_info**: returns basic channel setup information

5. **get_logs**: returns the latest HTML and pictures recorded
   - Required Inputs:
     - crawl: item type source
       - Facebook
       - Ebay
       - Default

6. **get**: get item prices and details
   - Required Inputs:
     - Name: item name

7. **set_price**: removes all trackers and manually sets price
   - Required Inputs:
     - Name: item name
     - price: desired price

8. **suppress**: suppress notifications for an item
   - Required Inputs:
     - Name: item name
     - suppress: boolean, whether to suppress or not

9. **edit_timer**: edit the scraping interval for an item
   - Required Inputs:
     - Name: item name
     - timer: new timer (in hours)

10. **edit_name**: edit item name (used for eBay queries)
    - Required Inputs:
      - old_name: current item name
      - new_name: new item name

11. **add_additional_name**: add additional names for tracking regex
    - Required Inputs:
      - Name: item name
      - additional_name: additional name variant

12. **remove_alternative_name**: remove additional names from tracking regex
    - Required Inputs:
      - Name: item name
      - index: index of alternative name to remove

13. **add_exclusion_query**: add regex exclusion pattern for filtering listings
    - Required Inputs:
      - Name: item name
      - exclusion_query: regex pattern to exclude (e.g., "broken", "for parts")

14. **remove_exclusion_query**: remove regex exclusion pattern
    - Required Inputs:
      - Name: item name
      - index: index of exclusion query to remove

15. **edit_tracking**: edit an existing tracker
    - Subcommands:
      - **add**: add new pair of tracking URI and HTML
        - Required Inputs:
          - Name: item name
          - uri: scraping URI
          - html_tag: CSS query
      - **remove**: remove pair of tracking URI and HTML
        - Required Inputs:
          - Name: item name
          - uri: tracking URI index

16. **graph**: graph price history of an item
    - Required Inputs:
      - Name: item name
      - months: duration of history to display

17. **graph-compare**: compare price graphs of multiple items
    - Required Inputs:
      - Name1: first item name
      - Name2: second item name
      - months: duration of history to display

18. **aggregate**: get aggregate data for used listings of an item
    - Required Inputs:
      - Name: item name
      - months: duration of history to aggregate
      - ending_month: how many months ago the aggregation should end

19. **channel_item_summary_one_week**: get aggregate data for the past week

20. **channel_item_summary_custom_ln**: get aggregate data for custom time period
    - Required Inputs:
      - months: how many months back for the aggregations

21. **edit_facebook_crawl**: toggle Facebook marketplace crawling for an item
    - Required Inputs:
      - Name: item name
      - crawl: boolean to enable/disable

22. **setup**: setup new discord channel for the bot
    - Required Inputs:
      - location: marketplace location (format: City Name, State)
      - marketplace-location-code: location code the marketplace uses
      - distance: maximum marketplace search distance

23. **get_failure_report**: get crawler failure analytics with visual charts
    - Required Inputs:
      - days: number of days of history to include

24. **restart**: saves progress and stops the bot

25. **help**: show help and available commands

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
