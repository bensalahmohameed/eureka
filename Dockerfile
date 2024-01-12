# Use a base image with Java and a minimal Linux distribution
FROM openjdk:17-jdk-alpine

# Set the working directory inside the container
WORKDIR /app

# Copy the JAR file into the container at /app
COPY target/EurekaRegistry-service-0.0.1-SNAPSHOT.jar /app/

# Specify the command to run on container startup
CMD ["java", "-jar", "EurekaRegistry-service-0.0.1-SNAPSHOT.jar"]

