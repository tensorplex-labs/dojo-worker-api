import traceback

import pandas as pd


def _get_prefix_till_first_param(query: str) -> tuple[bool, str]:
    """
    Get the prefix of the query till the first parameter.
    """
    param_index = query.find("$1")
    if param_index != -1:
        has_param = False
        return has_param, query[:param_index]
    return True, query


def dedupe_grafana_prisma_sql_logs(file_path, output_file_path):
    """
    This function reads a CSV file with development logs, removes duplicate JSON entries in the "Line" column,
    and writes the distinct queries to a new CSV file.

    :param file_path: str - The path to the CSV file to be read.
    :param output_file_path: str - The path where the deduplicated CSV file will be saved.
    :return: DataFrame - A DataFrame containing the deduplicated log data.
    """
    try:
        data = pd.read_csv(file_path)

        # Check if 'Line' column exists
        if "Line" in data.columns:
            # Parse JSON objects and deduplicate based on 'fields' key
            unique_queries = {}
            for _, row in data.iterrows():
                log_data = row["Line"]
                import json

                log_data_json = json.loads(log_data)
                query = log_data_json["fields"]["query"]
                has_param, prefix = _get_prefix_till_first_param(query)
                # check if the query has a parameter
                if has_param:
                    if prefix not in unique_queries:
                        unique_queries[prefix] = row
                else:
                    if query not in unique_queries:
                        unique_queries[query] = row

            # Create a new DataFrame from the unique lines
            data = pd.DataFrame(unique_queries.values())

            # Save the deduplicated data to a new CSV file
            data.to_csv(output_file_path, index=False)

        return data
    except FileNotFoundError:
        print(f"File not found: {file_path}")
    except pd.errors.EmptyDataError:
        print("No data: The file is empty")
    except pd.errors.ParserError:
        print("Parsing error: Could not parse the CSV file")
    except Exception as e:
        traceback.print_exc()
        print(f"An error occurred: {e}")


unique_queries = dedupe_grafana_prisma_sql_logs(
    "DEV LOGS (SQL)-data-2024-11-26 11_12_02.csv", "sql_queries_deduped.csv"
)
