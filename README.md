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

## Setup Guide

I'll make this later I'm too lazy right now

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

6. **get**: get all links for an item
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

12. **edit_tracking**: edit an existing tracker
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

13. **graph**: graph price history of an item
    - Required Inputs:
      - Name: item name
      - months: duration of history to display

14. **graph-compare**: compare price graphs of multiple items
    - Required Inputs:
      - Name1: first item name
      - Name2: second item name
      - months: duration of history to display

15. **aggregate**: get aggregate data for used listings of an item
    - Required Inputs:
      - Name: item name
      - months: duration of history to aggregate
      - ending_month: how many months ago the aggregation should end

16. **channel_item_summary_one_week**: get aggregate data for the past week

17. **channel_item_summary_custom_ln**: get aggregate data for custom time period
    - Required Inputs:
      - months: how many months back for the aggregations

18. **setup**: create new tracking table
    - Required Inputs:
      - location: marketplace location (format: City Name, State)
      - marketplace-location-code: location code the marketplace uses
      - distance: maximum marketplace search distance

19. **restart**: saves progress and stops the bot

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
